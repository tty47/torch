package k8s

import (
	"fmt"
	"github.com/jrmanes/torch/config"
)

var (
	trustedPeerFile          = "/tmp/TP-ADDR"
	trustedPeerFileConsensus = "/home/celestia/config/TP-ADDR"
	trustedPeers             = "/tmp/"
	cmd                      = `$(ifconfig | grep -oE 'inet addr:([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)' | grep -v '127.0.0.1' | awk '{print substr($2, 6)}')`
	trustedPeerPrefix        = "/ip4/" + cmd + "/tcp/2121/p2p/"
)

// GetTrustedPeersPath get the peers path from config or return the default value
func GetTrustedPeersPath(cfg config.MutualPeer) string {
	// if not defined in the config, return the default value
	if cfg.TrustedPeersPath == "" {
		return trustedPeers
	}

	return cfg.TrustedPeersPath
}

// GetTrustedPeerCommand generates the command for retrieving trusted peer information.
func GetTrustedPeerCommand() []string {
	script := fmt.Sprintf(`#!/bin/sh
# add the prefix to the addr
if [ -f "%[1]s" ];then
  cat "%[1]s"
fi`, trustedPeerFile)

	return []string{"sh", "-c", script}
}

// CreateFileWithEnvVar creates the file in the FS with the node to connect
func CreateFileWithEnvVar(nodeToFile string) []string {
	script := fmt.Sprintf(`
	#!/bin/sh
	echo -n "%[2]s" > "%[1]s"
	`, trustedPeerFileConsensus, nodeToFile)

	return []string{"sh", "-c", script}
}

// CreateTrustedPeerCommand generates the command for creating trusted peers.
// we have to use the shell script because we can only get the token and the
// nodeID from the node itself
func CreateTrustedPeerCommand() []string {
	script := fmt.Sprintf(`#!/bin/sh
if [ -f "%[1]s" ];then
  cat "%[1]s"
else
 # add the prefix to the addr
  echo -n "%[2]s" > "%[1]s"

  # generate the token
  export AUTHTOKEN=$(celestia bridge auth admin --node.store /home/celestia)

  # remove the first warning line...
  export AUTHTOKEN=$(echo $AUTHTOKEN|rev|cut -d' ' -f1|rev)

  # make the request and parse the response
  TP_ADDR=$(wget --header="Authorization: Bearer $AUTHTOKEN" \
       --header="Content-Type: application/json" \
       --post-data='{"jsonrpc":"2.0","id":0,"method":"p2p.Info","params":[]}' \
       --output-document - \
       http://localhost:26658 | grep -o '"ID":"[^"]*"' | sed 's/"ID":"\([^"]*\)"/\1/')
  
  echo -n "${TP_ADDR}" >> "%[1]s"
  cat "%[1]s"
fi`, trustedPeerFile, trustedPeerPrefix)

	return []string{"sh", "-c", script}
}

// BulkTrustedPeerCommand generates the peers content in the files
func BulkTrustedPeerCommand(tp string, cfg config.MutualPeer) []string {
	// Get the path to write
	trustedPeers = GetTrustedPeersPath(cfg)

	script := fmt.Sprintf(`#!/bin/sh
# create the folder if doesnt exists
mkdir -p "%[3]s"

if [ ! -f "%[3]s" ];then
  cp "%[2]s" "%[3]s/TRUSTED_PEERS"
fi
# Generate Trusteed Peers only if they are not in the file
grep -qF "%[1]s" "%[3]s/TRUSTED_PEERS" || echo ",%[1]s" >> "%[3]s/TRUSTED_PEERS"
`, tp, trustedPeerFile, trustedPeers)
	return []string{"sh", "-c", script}
}
