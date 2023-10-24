package nodes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jrmanes/torch/config"
	"github.com/jrmanes/torch/pkg/db/redis"
	"github.com/jrmanes/torch/pkg/k8s"
	"github.com/jrmanes/torch/pkg/metrics"

	log "github.com/sirupsen/logrus"
)

var (
	daContainerSetupName = "da-setup"                     // daContainerSetupName initContainer that we use to configure the nodes.
	daContainerName      = "da"                           // daContainerName container name which the pod runs.
	fPathDA              = "/tmp/celestia-config/TP-ADDR" // fPathDA path to the file where Torch will write.
)

// SetDaNodeDefault sets all the default values in case they are empty
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
			return err
		}

		// check if the MA is already in the config
		c, addPrefix = VerifyAndUpdateMultiAddress(peer, i, c, addPrefix)

		// if the node is not in the db, then we generate it
		if c == "" {
			log.Info("Node ", "["+s+"]"+" NOT found in DB, let's generate it")
			c, err = GenerateNodeIdAndSaveIt(peer, peer.ConnectsTo[i], red, ctx)
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

		// validate the MA, must start with /ip4/ || /dns/
		if !strings.HasPrefix(c, "/ip4/") && !strings.HasPrefix(c, "/dns/") {
			errorMessage := fmt.Sprintf("Error generating the MultiAddress, must begin with /ip4/ || /dns/: [%s]", c)
			log.Error(errorMessage)
			return errors.New(errorMessage)
		}

		log.Info("Registering metric for node: [", s, "]")

		// Register a multi-address metric
		m := metrics.MultiAddrs{
			ServiceName: "torch",
			NodeName:    s,
			MultiAddr:   c,
			Namespace:   k8s.GetCurrentNamespace(),
			Value:       1,
		}
		k8s.RegisterMetric(m)

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

		log.Info("Adding node to the queue: [", peer.NodeName, "]")
		go AddToQueue(peer)
	}

	return nil
}

// VerifyAndUpdateMultiAddress checks if the configuration contains a Multi Address at the specified index
// and updates it if found. It returns the verified Multi Address and a boolean indicating if an update was performed.
func VerifyAndUpdateMultiAddress(peer config.Peer, index int, currentAddr string, addPrefix bool) (string, bool) {
	// verify that we have the multi addr already specify in the config
	if strings.Contains(peer.ConnectsTo[index], "dns") || strings.Contains(peer.ConnectsTo[index], "ip4") {
		// Use the address from the configuration
		currentAddr = peer.ConnectsTo[index]
		addPrefix = false
	}
	return currentAddr, addPrefix
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
	connNode string,
	red *redis.RedisClient,
	ctx context.Context,
) (string, error) {
	// Generate the command and run it against the connection node + it's running container
	command := k8s.CreateTrustedPeerCommand()
	output, err := k8s.RunRemoteCommand(
		connNode,
		pod.ContainerName,
		k8s.GetCurrentNamespace(),
		command)
	if err != nil {
		log.Error("Error executing remote command: ", err)
		return "", err
	}

	if output != "" {
		log.Info("Adding pod id to Redis: ", connNode, " [", output, "] ")
		// save node in redis
		err = redis.SaveNodeId(connNode, red, ctx, output)
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
