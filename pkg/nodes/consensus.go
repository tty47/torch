package k8s

import (
	"encoding/json"
	"fmt"
	"github.com/jrmanes/torch/config"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

// GenesisHash
func GenesisHash(pods config.MutualPeersConfig) (string, string) {
	consensusNode := pods.MutualPeers[0].ConsensusNode
	url := fmt.Sprintf("http://%s:26657/block?height=1", consensusNode)

	response, err := http.Get(url)
	if err != nil {
		log.Error("Error making GET request:", err)
		return "", ""
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		log.Error("Non-OK response:", response.Status)
		return "", ""
	}

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Error("Error reading response body:", err)
		return "", ""
	}

	bodyString := string(bodyBytes)
	log.Info("Response Body: ", bodyString)

	// Parse the JSON response into a generic map
	var jsonResponse map[string]interface{}
	err = json.Unmarshal([]byte(bodyString), &jsonResponse)
	if err != nil {
		log.Error("Error parsing JSON:", err)
		return "", ""
	}

	// Access and print the .block_id.hash field
	blockIDHash, ok := jsonResponse["result"].(map[string]interface{})["block_id"].(map[string]interface{})["hash"].(string)
	if !ok {
		log.Error("Unable to access .block_id.hash")
		return "", ""
	}

	// Access and print the .block.header.time field
	blockTime, ok := jsonResponse["result"].(map[string]interface{})["block"].(map[string]interface{})["header"].(map[string]interface{})["time"].(string)
	if !ok {
		log.Error("Unable to access .block.header.time")
		return "", ""
	}

	log.Info("Block ID Hash: ", blockIDHash)
	log.Info("Block Time: ", blockTime)
	log.Info("Full output: ", bodyString)

	return blockIDHash, blockTime
}
