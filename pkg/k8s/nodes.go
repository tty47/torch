package k8s

import (
	"context"
	"errors"
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

// validateNode checks if an input node is available in the config.
func validateNode(n string, cfg config.MutualPeersConfig) (bool, string, string) {
	// check if the node received by the request is on the list, if so, we
	// continue the process
	for _, mutualPeer := range cfg.MutualPeers {
		for _, peer := range mutualPeer.Peers {
			if peer.NodeName == n {
				log.Info("Pod found in the config, executing remote command...")
				return true, peer.NodeName, peer.ContainerName
			}
		}
	}

	return false, "", ""
}

// GenerateTrustedPeersAddr handles the HTTP request to generate trusted peers' addresses.
func GenerateTrustedPeersAddr(cfg config.MutualPeersConfig, p config.Peer) (string, error) {
	// validate if the node received is ok
	ok, pod, cont := validateNode(p.NodeName, cfg)
	if !ok {
		log.Error("Pod name not valid: ", pod)
		return "", errors.New("pod name not valid")
	}

	// initialize the variables
	red := redis.InitRedisConfig()
	ctx := context.TODO()
	output := ""

	// check if the ide is already in the DB
	nodeId, err := CheckIfNodeExistsInDB(red, ctx, pod)
	if err != nil {
		log.Error("Error getting the node from db -> CheckIfNodeExistsInDB: ", err)
	}

	// if the node is already in the db, we just return it, otherwise, we generate them
	if nodeId != "" {
		log.Info("Pod ID found in Redis: [" + nodeId + "]")
		output = nodeId
	} else {
		log.Info("Pod ID not found, let's generate the id: [", pod, "] ", cont, " ns: [", GetCurrentNamespace(), "]")

		err = SetEnvVarInNodes(p, cfg)
		if err != nil {
			log.Error("error while SetEnvVarInNodes: ", err)
			return "", err
		}
		// read the connection list
		// get the ids
		// generate the string
		// write it to the node

		// get the command
		//command := CreateTrustedPeerCommand()
		//output, err = RunRemoteCommand(
		//	pod,
		//	cont,
		//	GetCurrentNamespace(),
		//	command)
		//if err != nil {
		//	log.Error("Error executing remote command: ", err)
		//	return "", err
		//}
		//
		//log.Info("Adding pod id to Redis: ", pod, " [", output, "] ")
		//// save node in redis
		//err := SaveNodeId(pod, red, ctx, output)
		//if err != nil {
		//	log.Error("Error SaveNodeId: ", err)
		//}
	}

	// Registering metric
	m := metrics.MultiAddrs{
		ServiceName: "torch",
		NodeName:    pod,
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

	nodeId, err := CheckIfNodeExistsInDB(r, ctx, peer.NodeName)
	if err != nil {
		log.Error("Error getting the node from db -> CheckIfNodeExistsInDB: ", err)
		return err
	}

	if nodeId != "" {
		log.Info("The node is already in Redis, we don't need to get it again: ", nodeId)
		return nil
	}

	err = SetEnvVarInNodes(peer, cfg)
	if err != nil {
		return err
	}

	if len(peer.DnsConnections) > 0 {
		trustedPeerPrefix = "/dns/" + strings.TrimSuffix(peer.NodeName, "-0") + "/tcp/2121/p2p/"
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
		err := SaveNodeId(peer.NodeName, r, ctx, output)
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

// SetEnvVarInNodes configure the ENV vars for those nodes that need it
func SetEnvVarInNodes(peer config.Peer, cfg config.MutualPeersConfig) error {
	// Setup nodes, check the type and generate the file to connect via ENV Var
	// Configure Consensus - connected using env var
	if peer.NodeType == "consensus" && peer.ConnectsAsEnvVar {
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

	// Configure DA Bridge Node
	if peer.NodeType == "da" && peer.ConnectsAsEnvVar {
		_, err := RunRemoteCommand(
			peer.NodeName,
			peer.ContainerSetupName,
			GetCurrentNamespace(),
			CreateFileWithEnvVar(peer.ConnectsTo[0], peer.NodeType),
		)
		if err != nil {
			log.Error("Error executing remote command: ", err)
			return err
		}
	}

	if peer.NodeType == "da" && !peer.ConnectsAsEnvVar {
		err := SetupDANodeWithConnections(peer, cfg)
		if err != nil {
			return err
		}
	}

	return nil
}
