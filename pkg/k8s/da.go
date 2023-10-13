package k8s

import (
	"context"

	"github.com/jrmanes/torch/config"
	"github.com/jrmanes/torch/pkg/db/redis"

	log "github.com/sirupsen/logrus"
)

// SetupDANodeWithConnections configure a DA node with connections
func SetupDANodeWithConnections(peer config.Peer, cfg config.MutualPeersConfig) error {
	red := redis.InitRedisConfig()
	ctx := context.TODO()

	log.Info("This is a da-full/light-node")
	connString := ""
	dnsConn := ""

	for i, s := range peer.ConnectsTo {
		log.Info("connection: ", i, " to: ", s)
		c, err := CheckIfNodeExistsInDB(red, ctx, s)
		if err != nil {
			log.Error("Error CheckIfNodeExistsInDB for full-node: [", peer.NodeName, "]", err)
		}

		// if the node is not in the db, then we add it
		if c == "" {
			log.Info("Node ", "["+s+"]"+" is empty, let's generate it")
			if len(peer.DnsConnections) > 0 {
				dnsConn = peer.DnsConnections[i]
			}
			_, err := GenerateNodeIdAndSaveIt(s, dnsConn, cfg, red, ctx)
			if err != nil {
				log.Error("Error GenerateNodeIdAndSaveIt for full-node: [", peer.NodeName, "]", err)
				return err
			}
		} else {
			log.Info("Node ", "["+s+"]"+" id: ", c)
			// add the next one
			if i > 0 {
				connString = connString + "," + c
			} else {
				connString = c
			}
		}
	}

	log.Info("multiaddr is: ", connString)
	command := WriteToFile(connString, trustedPeerFile)

	output, err := RunRemoteCommand(
		peer.NodeName,
		peer.ContainerSetupName,
		GetCurrentNamespace(),
		command)
	if err != nil {
		log.Error("Error executing remote command: ", err)
		return err
	}
	log.Info("output is: ", output)

	// TODO:
	// write the multi address to the node
	return nil
}

// GenerateNodeIdAndSaveIt generates the node id and store it
func GenerateNodeIdAndSaveIt(
	pod,
	dnsConn string,
	cfg config.MutualPeersConfig,
	red *redis.RedisClient,
	ctx context.Context,
) (string, error) {
	// Generate the command and run it
	command := CreateTrustedPeerCommand(dnsConn)

	output, err := RunRemoteCommand(
		pod,
		GetContainerInPod(pod, cfg),
		GetCurrentNamespace(),
		command)
	if err != nil {
		log.Error("Error executing remote command: ", err)
		return "", err
	}

	log.Info("Adding pod id to Redis: ", pod, " [", output, "] ")
	// save node in redis
	err = SaveNodeId(pod, red, ctx, output)
	if err != nil {
		log.Error("Error SaveNodeId: ", err)
	}

	return output, nil
}

// GetContainerInPod returns the container name from a pod based on the config file
func GetContainerInPod(pod string, cfg config.MutualPeersConfig) string {
	for _, mutualPeer := range cfg.MutualPeers {
		for _, peer := range mutualPeer.Peers {
			if pod == peer.NodeName {
				log.Info("Container name is: ", peer.ContainerName)
				return peer.ContainerName
			}
		}
	}
	return ""
}
