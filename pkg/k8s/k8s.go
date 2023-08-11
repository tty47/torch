package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/jrmanes/torch/config"
	"github.com/jrmanes/torch/pkg/metrics"

	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// GenerateList generates a list of matching pods based on the configured NodeName values.
func GenerateList(cfg config.MutualPeersConfig) []string {
	// matchingPods Stores the matching pods.
	var matchingPods []string

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error %v", err)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("Error %v", err)
	}

	log.Info("Namespace: ", GetCurrentNamespace())

	// get pods in the current namespace
	pods, err := clientset.CoreV1().Pods(GetCurrentNamespace()).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error("Failed to get pods:", err)
	}

	log.Info("There are ", len(pods.Items), " pods in the namespace")

	// Check if the pod names match the configured NodeName values
	for _, pod := range pods.Items {
		podName := pod.Name
		for _, mutualPeer := range cfg.MutualPeers {
			for _, peer := range mutualPeer.Peers {
				if podName == peer.NodeName {
					log.Info("Pod matches the name: ", pod.Name, " ", peer.NodeName)
					matchingPods = append(matchingPods, podName)
				}
			}
		}
	}

	// matchingPods Stores the matching pods.
	return matchingPods
}

// GenerateTrustedPeersAddr handles the HTTP request to generate trusted peers' addresses.
func GenerateTrustedPeersAddr(cfg config.MutualPeersConfig, pod string) (string, error) {
	// get the command
	command := CreateTrustedPeerCommand()

	// validate if the node received is ok
	ok, pod, cont := validateNode(pod, cfg)
	if !ok {
		log.Error("Pod name not valid", pod)
		return "", errors.New("Pod name not valid...")
	}

	log.Info("Pod found: ", pod, " ", cont, " ", GetCurrentNamespace())

	output, err := RunRemoteCommand(
		pod,
		cont,
		GetCurrentNamespace(),
		command)
	if err != nil {
		log.Error("Error executing remote command: ", err)
		return "", err
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
	// get the command
	command := CreateTrustedPeerCommand()

	// Create a map to store the pod names
	podMap := make(map[string]bool)

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

					output, err := RunRemoteCommand(
						peer.NodeName,
						peer.ContainerName,
						GetCurrentNamespace(),
						command)
					if err != nil {
						log.Error("Error executing remote command: ", err)
						// Handle the error or add it to a shared error channel
						return
					}

					StoreNodeIDs(peer.NodeName, output)

					m := metrics.MultiAddrs{
						ServiceName: "torch",
						NodeName:    peer.NodeName,
						MultiAddr:   output,
						Namespace:   GetCurrentNamespace(),
						Value:       1,
					}
					RegisterMetric(m)
				}(peer)
			}
		}
	}

	wg.Wait()

	// generate the data on the nodes by calling BulkTrusteedPeers
	for _, mutualPeer := range cfg.MutualPeers {
		for _, peer := range mutualPeer.Peers {
			// Check if the peer's NodeName is present in the podMap
			if _, exists := podMap[peer.NodeName]; exists {
				log.Info("Generating config for node:", peer.NodeName)
				BulkTrustedPeers(*mutualPeer)
				break // Skip to the next mutualPeer
			}
		}
	}

	return nodeIDsMap, nil
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
	c := exec.Command("wget", "-q", "-O", "-", fmt.Sprintf("http://%s:26657/block?height=1", consensusNode))

	// Create a buffer to capture the command's output
	var outputBuffer bytes.Buffer
	c.Stdout = &outputBuffer

	// Run the command
	err := c.Run()
	if err != nil {
		log.Error("Error:", err)
		return "", ""
	}

	// Convert the output buffer to a string
	outputString := outputBuffer.String()

	// Parse the JSON response into a generic map
	var response map[string]interface{}
	err = json.Unmarshal([]byte(outputString), &response)
	if err != nil {
		log.Error("Error parsing JSON:", err)
		return "", ""
	}

	// Access and print the .block_id.hash field
	blockIDHash, ok := response["result"].(map[string]interface{})["block_id"].(map[string]interface{})["hash"].(string)
	if !ok {
		log.Error("Unable to access .block_id.hash")
		return "", ""
	}

	// Access and print the .block.header.time field
	blockTime, ok := response["result"].(map[string]interface{})["block"].(map[string]interface{})["header"].(map[string]interface{})["time"].(string)
	if !ok {
		log.Error("Unable to access .block.header.time")
		return "", ""
	}

	log.Info("Block ID Hash: ", blockIDHash)
	log.Info("Block Time: ", blockTime)
	log.Info("Full output: ", outputString)

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
