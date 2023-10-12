package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jrmanes/torch/config"
	"github.com/jrmanes/torch/pkg/db/redis"
	"github.com/jrmanes/torch/pkg/metrics"

	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type NodeAddress struct {
	ID       string
	NodeName string
}

var nodeIDsMap map[string]string

// GetCurrentNamespace gets the current namespace from the environment variable.
// If the variable is not defined, the default value "default" is used.
func GetCurrentNamespace() string {
	// currentNamespace Stores the current namespace.
	currentNamespace := os.Getenv("POD_NAMESPACE")
	if currentNamespace == "" {
		log.Warn("Current Namespace variable is not defined, using the default value")
		return "default"
	}
	return currentNamespace
}

// StoreNodeIDs stores the nodeName-address pair in the map
func StoreNodeIDs(nodeName, id string) {
	// check if the nodeIDsMap has been initialized
	if nodeIDsMap == nil {
		nodeIDsMap = make(map[string]string)
	}
	nodeIDsMap[nodeName] = id
}

// GetAllIDs returns the nodeIDsMap
func GetAllIDs() map[string]string {
	return nodeIDsMap
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
func GenerateTrustedPeersAddr(cfg config.MutualPeersConfig, pod string) (string, error) {
	red := redis.InitRedisConfig()
	ctx := context.TODO()
	output := ""

	// validate if the node received is ok
	ok, pod, cont := validateNode(pod, cfg)
	if !ok {
		log.Error("Pod name not valid: ", pod)
		return "", errors.New("Pod name not valid...")
	}

	// check if the ide is already in the DB
	nodeId, err := CheckIfNodeExistsInDB(red, ctx, pod)
	if err != nil {
		log.Error("Error getting the node from db -> CheckIfNodeExistsInDB: ", err)
	}
	if nodeId != "" {
		log.Info("Pod ID found in Redis: [" + nodeId + "]")
		output = nodeId
	} else {
		log.Info("Pod ID not found, let's generate the id: ", pod, " ", cont, " ", GetCurrentNamespace())

		// get the command
		command := CreateTrustedPeerCommand()
		output, err = RunRemoteCommand(
			pod,
			cont,
			GetCurrentNamespace(),
			command)
		if err != nil {
			log.Error("Error executing remote command: ", err)
			return "", err
		}

		log.Info("Adding pod id to Redis: ", pod, " [", output, "] ")
		// save node in redis
		err := SaveNodeId(pod, red, ctx, output)
		if err != nil {
			log.Error("Error SaveNodeId: ", err)
		}
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

					err := GenerateAndRegisterTP(peer, red, ctx)
					if err != nil {
						log.Error("Error with GenerateAndRegisterTP: ", err)
					}

					if peer.NodeType == "da" {
						log.Info("Generating config for node:", peer.NodeName)
						BulkTrustedPeers(*mutualPeer)
					}
				}(peer)
			}
		}
	}

	wg.Wait()

	return nodeIDsMap, nil
}

// GenerateAndRegisterTP generates trusted peers for a specific node and registers metrics.
//
// This function generates trusted peers for the specified node based on its type and
// executes remote commands to obtain the necessary information. It also registers
// metrics for "da" nodes.
func GenerateAndRegisterTP(peer config.Peer, r *redis.RedisClient, ctx context.Context) error {

	nodeId, err := CheckIfNodeExistsInDB(r, ctx, peer.NodeName)
	if err != nil {
		log.Error("Error getting the node from db -> CheckIfNodeExistsInDB: ", err)
	}
	log.Info("nodeID:", nodeId)
	if nodeId != "" {
		log.Info("The value is in Redis, we don't need to get it again: ", nodeId)
		return nil
	}

	err = SetEnvVarInNodes(peer)
	if err != nil {
		return err
	}

	// Get the command for generating trusted peers
	command := CreateTrustedPeerCommand()

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
		// Store it only when it's a "da" node
		StoreNodeIDs(peer.NodeName, output)

		// save node in redis
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
func SetEnvVarInNodes(peer config.Peer) error {
	red := redis.InitRedisConfig()
	ctx := context.TODO()

	// Setup nodes, check the type and generate the file to connect via ENV Var
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
		log.Info("This is a da-full-node")
		connString := ""

		for i, s := range peer.ConnectsTo {
			log.Info("connection: ", i, " to: ", s)
			c, err := CheckIfNodeExistsInDB(red, ctx, s)
			if err != nil {
				log.Error("Error CheckIfNodeExistsInDB for full-node: [", peer.NodeName, "]", err)
			}
			// add the next one
			if i > 0 {
				connString = connString + "," + c
			} else {
				connString = c
			}
		}
		log.Info("multiaddr is: ", connString)
	}

	return nil
}

// SaveNodeId stores the values in redis
func SaveNodeId(
	podName string,
	r *redis.RedisClient,
	ctx context.Context,
	output string,
) error {
	// try to get the value from redis
	// if the value is empty, then we add it
	nodeName, err := CheckIfNodeExistsInDB(r, ctx, podName)
	if err != nil {
		return err
	}

	// if the node is not in the db, then we add it
	if nodeName == "" {
		log.Info("Node ", "["+podName+"]"+" not found in Redis, let's add it")
		err := r.SetKey(ctx, podName, output, 1000*time.Hour)
		if err != nil {
			log.Error("Error adding the node to redis: ", err)
			return err
		}
	}

	return nil
}

// CheckIfNodeExistsInDB checks if node is in the DB and return it
func CheckIfNodeExistsInDB(
	r *redis.RedisClient,
	ctx context.Context,
	nodeName string,
) (string, error) {
	nodeName, err := r.GetKey(ctx, nodeName)
	if err != nil {
		log.Error("Error: ", err)
		return "", err
	}

	return nodeName, err
}

func BulkTrustedPeers(pods config.MutualPeer) {
	// Get the data from the map containing trusted peers' addresses
	data := GetAllIDs()

	// Create a channel to collect errors from goroutines
	errCh := make(chan error)
	// Use a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Loop through the trusted peers' addresses in the data map
	for key := range data {
		for _, pod := range pods.Peers {
			// Skip if the current trusted peer's address matches the current pod's NodeName
			if key != pod.NodeName {
				wg.Add(1)
				// Launch a goroutine to execute the remote command for the current pod
				go func(peer config.Peer) {
					defer wg.Done()

					// Generate the command to get trusted peers' addresses for the current pod
					command := BulkTrustedPeerCommand(data[key], pods)

					// Execute the remote command to get trusted peers' addresses
					output, err := RunRemoteCommand(
						peer.NodeName,
						peer.ContainerName,
						GetCurrentNamespace(),
						command)
					if err != nil {
						// If an error occurs, send it to the error channel
						errCh <- err
						return
					}
					log.Info("OUTPUT: ", output)

					// Generate the metrics with the MultiAddrs
					m := metrics.MultiAddrs{
						ServiceName: "torch",
						NodeName:    peer.NodeName,
						MultiAddr:   output,
						Namespace:   GetCurrentNamespace(),
						Value:       1,
					}
					RegisterMetric(m)
				}(pod)
			}
		}
	}

	// Close the error channel after all goroutines finish
	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Collect errors from the error channel and log them
	for err := range errCh {
		log.Error("Error executing remote command: ", err)
	}
}

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

// RunRemoteCommand executes a remote command on the specified node.
func RunRemoteCommand(nodeName, container, namespace string, command []string) (string, error) {
	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Error("Error: ", err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("Error: %v", err.Error())
	}

	// Create a request to execute the command on the specified node.
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(nodeName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Command:   command,
			Container: container,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	// Execute the remote command.
	output, err := executeCommand(clusterConfig, req)
	if err != nil {
		log.Error("failed to execute remote command: ", err)
	}

	return output, nil
}

// executeCommand executes the remote command using the provided configuration, request, and output writer.
func executeCommand(config *rest.Config, req *rest.Request) (string, error) {
	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		log.Error("failed to create SPDY executor: ", err)
	}

	// Prepare the standard I/O streams.
	var stdout, stderr bytes.Buffer

	// Execute the remote command and capture the output.
	err = executor.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		log.Error("failed to execute command stream: ", err)
	}

	return stdout.String(), nil
}
