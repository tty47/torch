package nodes

import (
	log "github.com/sirupsen/logrus"

	"github.com/jrmanes/torch/config"
	"github.com/jrmanes/torch/pkg/k8s"
)

type NodeAddress struct {
	ID       string
	NodeName string
}

// ValidateNode checks if a node received is available in the config, meaning that we can proceed to generate it is id.
// if not, we return an error and an empty node struct.
func ValidateNode(n string, cfg config.MutualPeersConfig) (bool, config.Peer) {
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
