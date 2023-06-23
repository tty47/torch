package config

// MutualPeersConfig represents the configuration structure.
type MutualPeersConfig struct {
	// List of mutual peers.
	MutualPeers []*MutualPeer `yaml:"mutualPeers"`
}

// MutualPeer represents a mutual peer structure.
type MutualPeer struct {
	// List of peers.
	Peers            []Peer `yaml:"peers"`
	TrustedPeersPath string `yaml:"trustedPeersPath"` // New field
}

// Peer represents a peer structure.
type Peer struct {
	// NodeName of the peer node.
	NodeName      string `yaml:"nodeName"`
	ContainerName string `yaml:"containerName"`
}
