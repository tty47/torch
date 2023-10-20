package nodes

import (
	"context"
	"strings"

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

// SetDaNodeDefault sets the default values in case they are empty
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
func SetupDANodeWithConnections(peer config.Peer) error {
	red := redis.InitRedisConfig()
	ctx := context.TODO()
	connString := ""
	addPrefix := true

	// read the connection list
	for i, s := range peer.ConnectsTo {
		log.Info(peer.NodeName, " , connection: [", i, "] to node: [", s, "]")

		// checking the node in the DB first
		c, err := redis.CheckIfNodeExistsInDB(red, ctx, s)
		if err != nil {
			log.Error("Error CheckIfNodeExistsInDB for full-node: [", peer.NodeName, "]", err)
		}

		// check if the MA is already in the config
		c, addPrefix = HasAddrAlready(peer, i, c, addPrefix)

		// if the node is not in the db, then we generate it
		if c == "" {
			log.Info("Node ", "["+s+"]"+" NOT found in DB, let's generate it")
			c, err = GenerateNodeIdAndSaveIt(peer, i, red, ctx)
			if err != nil {
				log.Error("Error GenerateNodeIdAndSaveIt for full-node: [", peer.NodeName, "]", err)
				return err
			}
		}

		// if we have the address already, lets continue the process, otherwise, means we couldn't get the node id
		if c != "" && addPrefix {
			// adding the node prefix
			c, err = SetIdPrefix(peer, c, i)
			if err != nil {
				log.Error("Error SetIdPrefix for full-node: [", peer.NodeName, "]", err)
				return err
			}
			log.Info("Peer connection prefix: ", c)
		}

		// check the connection index and concatenate it in case we have more than one node
		if i > 0 {
			connString = connString + "," + c
		} else {
			connString = c
		}

		// get the command to write in a file and execute the command against the node
		command := k8s.WriteToFile(connString, fPathDA)
		output, err := k8s.RunRemoteCommand(
			peer.NodeName,
			peer.ContainerSetupName,
			k8s.GetCurrentNamespace(),
			command)
		if err != nil {
			log.Error("Error executing remote command: ", err)
			return err
		}

		log.Info("MultiAddr for node ", peer.NodeName, " is: [", output, "]")
	}

	return nil
}

func HasAddrAlready(peer config.Peer, i int, c string, addPrefix bool) (string, bool) {
	// verify that we have the multi addr already specify in the config
	if strings.Contains(peer.ConnectsTo[i], "dns") || strings.Contains(peer.ConnectsTo[i], "ip4") {
		c = peer.ConnectsTo[i]
		addPrefix = false
	}
	return c, addPrefix
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
