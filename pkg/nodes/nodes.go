package nodes

import (
	"context"
	"github.com/celestiaorg/torch/config"
	"github.com/celestiaorg/torch/pkg/db/redis"
	"github.com/celestiaorg/torch/pkg/k8s"
	"github.com/celestiaorg/torch/pkg/metrics"

	log "github.com/sirupsen/logrus"
)

type NodeAddress struct {
	ID       string
	NodeName string
}

// ValidateNode checks if an input node is available in the config.
func ValidateNode(n string, cfg config.MutualPeersConfig) (bool, config.Peer) {
	// check if the node received by the request is on the list, if so, we
	// continue the process
	for _, mutualPeer := range cfg.MutualPeers {
		for _, peer := range mutualPeer.Peers {
			if peer.NodeName == n {
				log.Info("Pod found in the config, executing remote command...")
				return true, peer
			}
		}
	}

	return false, config.Peer{}
}

//// GenerateAllTrustedPeersAddr handles the HTTP request to generate trusted peers' addresses.
//func GenerateAllTrustedPeersAddr(cfg config.MutualPeersConfig, pod []string) (map[string]string, error) {
//	// Create a map to store the pod names
//	podMap := make(map[string]bool)
//
//	red := redis.InitRedisConfig()
//	ctx := context.TODO()
//
//	// Add the pod names to the map
//	for _, p := range pod {
//		podMap[p] = true
//	}
//
//	var wg sync.WaitGroup
//
//	for _, mutualPeer := range cfg.MutualPeers {
//		for _, peer := range mutualPeer.Peers {
//			if _, exists := podMap[peer.NodeName]; exists {
//				wg.Add(1)
//				go func(peer config.Peer) {
//					defer wg.Done()
//
//					err := GenerateAndRegisterTP(peer, cfg, red, ctx)
//					if err != nil {
//						log.Error("Error with GenerateAndRegisterTP: ", err)
//					}
//
//					if peer.NodeType == "da" {
//						log.Info("Generating config for node:", peer.NodeName)
//					}
//				}(peer)
//			}
//		}
//	}
//
//	wg.Wait()
//
//	keysAndValues, err := red.GetAllKeys(ctx)
//	if err != nil {
//		log.Error("Error getting the keys and values: ", err)
//	}
//
//	return keysAndValues, nil
//}

// GenerateAndRegisterTP generates trusted peers for a specific node and registers metrics.
//
// This function generates trusted peers for the specified node based on its type and
// executes remote commands to obtain the necessary information. It also registers
// metrics for "da" nodes.
func GenerateAndRegisterTP(
	peer config.Peer,
	cfg config.MutualPeersConfig,
	r *redis.RedisClient,
	ctx context.Context,
) error {
	nodeId, err := redis.CheckIfNodeExistsInDB(r, ctx, peer.NodeName)
	if err != nil {
		log.Error("Error getting the node from db -> CheckIfNodeExistsInDB: ", err)
		return err
	}

	if nodeId != "" {
		log.Info("The node is already in Redis, we don't need to get it again: ", nodeId)
		return nil
	}

	err = SetupNodesEnvVarAndConnections(peer, cfg)
	if err != nil {
		return err
	}

	if len(peer.DnsConnections) > 0 {
		log.Info("The node uses dns connections", peer.DnsConnections)
		//dnsConn = "/dns/" + strings.TrimSuffix(peer.NodeName, "-0") + "/tcp/2121/p2p/"
	} else {
		log.Info("the node use  IP")
	}

	// Get the command for generating trusted peers
	command := k8s.CreateTrustedPeerCommand()

	// Execute a remote command on the node
	output, err := k8s.RunRemoteCommand(
		peer.NodeName,
		peer.ContainerName,
		k8s.GetCurrentNamespace(),
		command)
	if err != nil {
		log.Error("Error executing remote command: ", err)
		// Handle the error or add it to a shared error channel
		return err
	}
	log.Info("output: ", output)

	// Register the metric only if the node is of type "da"
	if peer.NodeType == "da" {
		// save node in db
		err := redis.SaveNodeId(peer.NodeName, r, ctx, output)
		if err != nil {
			log.Error("Error SaveNodeId: ", err)
			return err
		}

		log.Info("Registering metric for node: [", peer.NodeName, "]")
		// Register a multi-address metric
		m := metrics.MultiAddrs{
			ServiceName: "torch",
			NodeName:    peer.NodeName,
			MultiAddr:   output,
			Namespace:   k8s.GetCurrentNamespace(),
			Value:       1,
		}
		k8s.RegisterMetric(m)
	}

	return nil
}

// SetupNodesEnvVarAndConnections configure the ENV vars for those nodes that needs to connect via ENV var
func SetupNodesEnvVarAndConnections(peer config.Peer, cfg config.MutualPeersConfig) error {
	// Configure Consensus & DA - connecting using env var
	_, err := k8s.RunRemoteCommand(
		peer.NodeName,
		peer.ContainerSetupName,
		k8s.GetCurrentNamespace(),
		k8s.CreateFileWithEnvVar(peer.ConnectsTo[0], peer.NodeType),
	)
	if err != nil {
		log.Error("Error executing remote command: ", err)
		return err
	}

	// check if the node is type DA, if so, add the node to the queue to generate the Multi Address later.
	if peer.NodeType == "da" {
		// we use the goroutine for that, otherwise, Torch tries to keep the connection opened.
		go AddToQueue(peer)
	}

	return nil
}
