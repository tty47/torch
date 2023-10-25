package nodes

import (
	"context"
	"time"

	"github.com/celestiaorg/torch/config"
	"github.com/celestiaorg/torch/pkg/db/redis"
	"github.com/celestiaorg/torch/pkg/k8s"
	"github.com/celestiaorg/torch/pkg/metrics"

	log "github.com/sirupsen/logrus"
)

var (
	taskQueue     = make(chan config.Peer) // taskQueue channel for pending tasks (peers to process later).
	MaxRetryCount = 5                      // MaxRetryCount number of retries per node.
)

// ProcessTaskQueue processes the pending tasks in the queue every 5 seconds.
func ProcessTaskQueue() {
	ticker := time.NewTicker(5 * time.Second) // Set the interval to 5 seconds
	//defer ticker.Stop()                       // Stop the ticker when the function exits

	for {
		select {
		case <-ticker.C:
			processQueue()
		}
	}
}

// processQueue process the nodes in the queue and tries to generate the Multi Address
func processQueue() {
	red := redis.InitRedisConfig()
	ctx := context.TODO()

	for {
		select {
		case peer := <-taskQueue:
			CheckNodesInDBOrCreateThem(peer, red, ctx)
		default:
			return
		}
	}
}

// CheckNodesInDBOrCreateThem try to find the node in the DB, if the node is not in the DB, it tries to create it.
func CheckNodesInDBOrCreateThem(peer config.Peer, red *redis.RedisClient, ctx context.Context) {
	log.Info("Processing Node in the queue: ", "[", peer.NodeName, "]")
	// check if the node is in the DB
	ma, err := redis.CheckIfNodeExistsInDB(red, ctx, peer.NodeName)
	if err != nil {
		log.Error("Error CheckIfNodeExistsInDB for node: [", peer.NodeName, "]", err)
	}

	// if the node doesn't exist in the DB, let's try to create it
	if ma == "" {
		log.Info("Node ", "["+peer.NodeName+"]"+" NOT found in DB, let's try to generate it")
		ma, err = GenerateNodeIdAndSaveIt(peer, peer.NodeName, red, ctx)
		if err != nil {
			log.Error("Error GenerateNodeIdAndSaveIt for full-node: [", peer.NodeName, "]", err)
		}
	}

	// check if the multi address is empty after trying to generate it
	if ma == "" {
		// check if the node is still under the maximum number of retries
		if peer.RetryCount < MaxRetryCount {
			log.Info("Node ", "["+peer.NodeName+"]"+" NOT found in DB, adding it to the queue, attempt: ", "[", peer.RetryCount, "]")
			peer.RetryCount++ // increment the counter
			AddToQueue(peer)
		} else {
			log.Info("Max retry count reached for node: ", "[", peer.NodeName, "]", "it might have some issues...")
		}
	} else {
		log.Info("Node ", "[", peer.NodeName, "]", " found in DB, ID: ", "[", ma, "]")
		// Register a multi-address metric
		m := metrics.MultiAddrs{
			ServiceName: "torch",
			NodeName:    peer.NodeName,
			MultiAddr:   ma,
			Namespace:   k8s.GetCurrentNamespace(),
			Value:       1,
		}
		k8s.RegisterMetric(m)
	}
}

// AddToQueue adds a function to add peers to the queue if necessary.
func AddToQueue(peer config.Peer) {
	peer.RetryCount = 0 // set the first attempt
	log.Info("Node added to the queue: ", peer)
	taskQueue <- peer
}
