package nodes

import (
	"context"

	"github.com/jrmanes/torch/config"
	"github.com/jrmanes/torch/pkg/db/redis"
	"github.com/jrmanes/torch/pkg/k8s"

	log "github.com/sirupsen/logrus"
)

var (
	daContainerSetupName = "da-setup"
	daContainerName      = "da"
	fPathDA              = "/tmp/celestia-config/TP-ADDR"
)

func SetDaNodeDefault(peer config.Peer) config.Peer {
	if peer.ContainerSetupName == "" {
		peer.ContainerSetupName = daContainerSetupName
	}
	if peer.ContainerName == "" {
		peer.ContainerName = daContainerName
	}
	return peer
}

// SetupDANodeWithConnections configure a DA node with connections
func SetupDANodeWithConnections(peer config.Peer, cfg config.MutualPeersConfig) error {
	red := redis.InitRedisConfig()
	ctx := context.TODO()
	connString := ""

	// read the connection list
	for i, s := range peer.ConnectsTo {
		log.Info("connection: ", i, " to: ", s)

		// checking the node in the DB first
		c, err := redis.CheckIfNodeExistsInDB(red, ctx, s)
		if err != nil {
			log.Error("Error CheckIfNodeExistsInDB for full-node: [", peer.NodeName, "]", err)
		}

		// if the node is not in the db, then we generate it
		if c == "" {
			log.Info("Node ", "["+s+"]"+" NOT found in DB, let's generate it")
			c, err = GenerateNodeIdAndSaveIt(peer, i, red, ctx)
			if err != nil {
				log.Error("Error GenerateNodeIdAndSaveIt for full-node: [", peer.NodeName, "]", err)
				return err
			}
		} else {
			log.Info("Node ", "["+s+"]"+" FOUND in DB: [", c, "]")
		}

		// if we have the address already, lets continue the process, otherwise, means we couldn't get the node id
		if c != "" {
			// adding the node prefix
			c, err = SetIdPrefix(peer, c, i)
			if err != nil {
				log.Error("Error SetIdPrefix for full-node: [", peer.NodeName, "]", err)
				return err
			}

			log.Info("Peer connection prefix: ", c)
			// add the next one
			if i > 0 {
				connString = connString + "," + c
			} else {
				connString = c
			}

			command := k8s.WriteToFile(connString, fPathDA)
			log.Debug("file to write in the node: ", fPathDA)
			log.Debug("peer.NodeName is: ", peer.NodeName)
			log.Debug("peer.ContainerSetupName is: ", peer.ContainerSetupName)
			log.Debug("command: ", command)

			output, err := k8s.RunRemoteCommand(
				peer.NodeName,
				peer.ContainerSetupName,
				k8s.GetCurrentNamespace(),
				command)
			if err != nil {
				log.Error("Error executing remote command: ", err)
				return err
			}

			log.Info("output is: ", output)
		} else {
			log.Info("Multi address not generated yet...")
		}
	}

	return nil
}

// SetIdPrefix generates the prefix depending on dns or ip
func SetIdPrefix(peer config.Peer, c string, i int) (string, error) {
	// check if we are using DNS or IP
	if len(peer.DnsConnections) > 0 {
		c = "/dns/" + peer.DnsConnections[i] + "/tcp/2121/p2p/" + c
	} else {
		comm := k8s.GetNodeIP()
		output, err := k8s.RunRemoteCommand(
			peer.ConnectsTo[i],
			peer.ContainerName,
			k8s.GetCurrentNamespace(),
			comm)
		if err != nil {
			log.Error("Error executing remote command: ", err)
			return "", err
		}
		log.Info("command - ip is: ", output)
		c = output + c
	}
	return c, nil
}

// GenerateNodeIdAndSaveIt generates the node id and store it
func GenerateNodeIdAndSaveIt(
	pod config.Peer,
	connNode int,
	red *redis.RedisClient,
	ctx context.Context,
) (string, error) {
	// Generate the command and run it
	command := k8s.CreateTrustedPeerCommand()

	output, err := k8s.RunRemoteCommand(
		pod.ConnectsTo[connNode],
		pod.ContainerName,
		k8s.GetCurrentNamespace(),
		command)
	if err != nil {
		log.Error("Error executing remote command: ", err)
		return "", err
	}

	if output != "" {
		log.Info("Adding pod id to Redis: ", pod.ConnectsTo[connNode], " [", output, "] ")
		// save node in redis
		err = redis.SaveNodeId(pod.ConnectsTo[connNode], red, ctx, output)
		if err != nil {
			log.Error("Error SaveNodeId: ", err)
			return "", err
		}
	} else {
		log.Error("Output is empty for pod: ", " [", pod.NodeName, "] ")
		return "", err
	}

	return output, nil
}
