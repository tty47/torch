package nodes

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jrmanes/torch/config"
	"github.com/jrmanes/torch/pkg/db/redis"
	"github.com/jrmanes/torch/pkg/metrics"
)

const (
	MaxRetryCount = 5               // MaxRetryCount number of retries per node.
	TickerTime    = 5 * time.Second // TickerTime time specified to make a signal.
)

var (
	taskQueue = make(chan config.Peer) // taskQueue channel for pending tasks (peers to process later).
)

// ProcessTaskQueue processes the pending tasks in the queue the time specified in the const TickerTime.
func ProcessTaskQueue(ctx context.Context) {
	ticker := time.NewTicker(TickerTime)

	for {
		select {
		case <-ctx.Done():
			// The context has been canceled, exit the loop.
			return
		case <-ticker.C:
			processQueue(ctx)
		}
	}
}

// processQueue process the nodes in the queue and tries to generate the Multi Address
func processQueue(ctx context.Context) {
	red := redis.InitRedisConfig()

	for {
		select {
		case <-ctx.Done():
			// The context has been canceled, exit the loop.
			return
		case peer := <-taskQueue:
			// Perform the operation with the node
			err := CheckNodesInDBOrCreateThem(peer, red, ctx)
			if err != nil {
				log.Error("Error checking the nodes: CheckNodesInDBOrCreateThem - ", err)
			}
		}
	}
} // CheckNodesInDBOrCreateThem attempts to find the node in the DB; if the node is not in the DB, it attempts to create it.
func CheckNodesInDBOrCreateThem(peer config.Peer, red *redis.RedisClient, ctx context.Context) error {
	log.Info("Processing Node in the queue: ", "[", peer.NodeName, "]")
	// Check if the node is in the DB
	ma, err := redis.CheckIfNodeExistsInDB(red, ctx, peer.NodeName)
	if err != nil {
		log.Error("Error CheckIfNodeExistsInDB for node: [", peer.NodeName, "]", err)
		return err
	}
	// If the node doesn't exist in the DB, attempt to generate it in a goroutine
	if ma == "" {
		log.Info("Node ", "["+peer.NodeName+"]"+" NOT found in DB, let's try to generate it")

		// Create a channel for errors
		errCh := make(chan error)

		// Start a goroutine for GenerateNodeIdAndSaveIt
		go func() {
			defer close(errCh)
			var generateErr error // Variable to store the error from GenerateNodeIdAndSaveIt
			ma, generateErr = GenerateNodeIdAndSaveIt(peer, peer.NodeName, red, ctx)
			if generateErr != nil {
				errCh <- generateErr
			}
		}()

		// Wait for the goroutine to finish and check for errors
		select {
		case generateErr := <-errCh:
			if generateErr != nil {
				log.Error("Error GenerateNodeIdAndSaveIt for full-node: [", peer.NodeName, "]", generateErr)
				return generateErr
			}
		case <-ctx.Done():
			// Context canceled, return an error or handle it as needed
			return ctx.Err()
		}
	}

	// Check if the multi-address is empty after attempting to generate it
	if ma == "" {
		// Check if the node is still within the maximum number of retries
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
			Namespace:   peer.Namespace,
			Value:       1,
		}
		metrics.RegisterMetric(m)
	}

	return nil
}

// AddToQueue adds a function to add peers to the queue if necessary.
func AddToQueue(peer config.Peer) {
	peer.RetryCount = 0 // set the first attempt
	log.Info("Node added to the queue: ", peer)
	taskQueue <- peer
}
