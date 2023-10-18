package k8s

import (
	"context"
	"github.com/jrmanes/torch/pkg/nodes"
	"strings"
	"sync"

	"github.com/jrmanes/torch/config"
	"github.com/jrmanes/torch/pkg/db/redis"
	"github.com/jrmanes/torch/pkg/metrics"

	log "github.com/sirupsen/logrus"
)

type NodeAddress struct {
	ID       string
	NodeName string
}

// ValidateNode checks if an input node is available in the config.
func ValidateNode(n string, cfg config.MutualPeersConfig) bool {
	// check if the node received by the request is on the list, if so, we
	// continue the process
	for _, mutualPeer := range cfg.MutualPeers {
		for _, peer := range mutualPeer.Peers {
			if peer.NodeName == n {
				log.Info("Pod found in the config, executing remote command...")
				return true
			}
		}
	}

	return false
}

// GenerateTrustedPeersAddr handles the HTTP request to generate trusted peers' addresses.
func GenerateTrustedPeersAddr(cfg config.MutualPeersConfig, p config.Peer) (string, error) {
	// initialize the variables
	red := redis.InitRedisConfig()
	ctx := context.TODO()
	output := ""

	// check if the ide is already in the DB
	nodeId, err := redis.CheckIfNodeExistsInDB(red, ctx, p.NodeName)
	if err != nil {
		log.Error("Error getting the node from db -> CheckIfNodeExistsInDB: ", err)
	}

	// if the node is already in the db, we just return it, otherwise, we generate them
	if nodeId != "" {
		log.Info("Pod ID found in Redis: [" + nodeId + "]")
		output = nodeId
	} else {
		log.Info("Pod ID not found, let's generate the id: [", p.NodeName, "] ", " container: [", p.ContainerName, "] ns: [", GetCurrentNamespace(), "]")

		err = SetupNodesEnvVarAndConnections(p, cfg)
		if err != nil {
			log.Error("error while SetupNodesEnvVarAndConnections: ", err)
			return "", err
		}
	}

	// if the node is not using env vars and it first connection is not the same as it's name, find it's connections
	if !p.ConnectsAsEnvVar && p.ConnectsTo[0] != p.NodeName {
		err := SetupNodesEnvVarAndConnections(p, cfg)
		if err != nil {
			log.Error("error while SetupNodesEnvVarAndConnections: ", err)
			return "", err
		}
	}

	// Registering metric
	m := metrics.MultiAddrs{
		ServiceName: "torch",
		NodeName:    p.NodeName,
		MultiAddr:   output,
		Namespace:   GetCurrentNamespace(),
		Value:       1,
	}
	RegisterMetric(m)

	return output, nil
}

// GenerateAllTrustedPeersAddr handles the HTTP request to generate trusted peers' addresses.
func GenerateAllTrustedPeersAddr(cfg config.MutualPeersConfig, pod []string) (map[string]string, error) {
	// Create a map to store the pod names
	podMap := make(map[string]bool)

	red := redis.InitRedisConfig()
	ctx := context.TODO()

	// Add the pod names to the map
	for _, p := range pod {
		podMap[p] = true
	}

	var wg sync.WaitGroup

	for _, mutualPeer := range cfg.MutualPeers {
		for _, peer := range mutualPeer.Peers {
			if _, exists := podMap[peer.NodeName]; exists {
				wg.Add(1)
				go func(peer config.Peer) {
					defer wg.Done()

					err := GenerateAndRegisterTP(peer, cfg, red, ctx)
					if err != nil {
						log.Error("Error with GenerateAndRegisterTP: ", err)
					}

					if peer.NodeType == "da" {
						log.Info("Generating config for node:", peer.NodeName)
					}
				}(peer)
			}
		}
	}

	wg.Wait()

	keysAndValues, err := red.GetAllKeys(ctx)
	if err != nil {
		log.Error("Error getting the keys and values: ", err)
	}

	return keysAndValues, nil
}

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
	dnsConn := ""

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
		dnsConn = "/dns/" + strings.TrimSuffix(peer.NodeName, "-0") + "/tcp/2121/p2p/"
	} else {
		log.Info("the node use  IP")
	}

	// Get the command for generating trusted peers
	command := CreateTrustedPeerCommand(dnsConn)

	// Execute a remote command on the node
	output, err := RunRemoteCommand(
		peer.NodeName,
		peer.ContainerName,
		GetCurrentNamespace(),
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

		// Register a multi-address metric
		m := metrics.MultiAddrs{
			ServiceName: "torch",
			NodeName:    peer.NodeName,
			MultiAddr:   output,
			Namespace:   GetCurrentNamespace(),
			Value:       1,
		}
		RegisterMetric(m)
	}

	return nil
}

// SetupNodesEnvVarAndConnections configure the ENV vars for those nodes that needs to connect via ENV var
func SetupNodesEnvVarAndConnections(peer config.Peer, cfg config.MutualPeersConfig) error {
	// Configure Consensus & DA - connecting using env var
	if peer.ConnectsAsEnvVar {
		_, err := RunRemoteCommand(
			peer.NodeName,
			peer.ContainerSetupName,
			GetCurrentNamespace(),
			CreateFileWithEnvVar(peer.ConnectsTo[0], peer.NodeType),
		)
		if err != nil {
			log.Error("Error executing remote command: ", err)
			// Handle the error or add it to a shared error channel
			return err
		}
	}

	// Configure DA Nodes with which are not using env var
	if peer.NodeType == "da" && !peer.ConnectsAsEnvVar {
		err := nodes.SetupDANodeWithConnections(peer, cfg)
		if err != nil {
			return err
		}
	}

	return nil
}
