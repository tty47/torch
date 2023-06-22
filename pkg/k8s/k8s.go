package k8s

import (
	"bytes"
	"context"
	"errors"
	"os"

	"github.com/jrmanes/mp-orch/config"

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

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
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

	// generate the storenodesids
	for _, mutualPeer := range cfg.MutualPeers {
		for _, peer := range mutualPeer.Peers {
			// Check if the peer's NodeName is present in the podMap
			if _, exists := podMap[peer.NodeName]; exists {
				log.Info("Generating config for node:", peer.NodeName)
				output, err := RunRemoteCommand(
					peer.NodeName,
					peer.ContainerName,
					GetCurrentNamespace(),
					command)
				if err != nil {
					log.Error("Error executing remote command: ", err)
					return nodeIDsMap, err
				}
				StoreNodeIDs(peer.NodeName, output)
			}
		}
	}

	// bulk the data
	for _, mutualPeer := range cfg.MutualPeers {
		for _, peer := range mutualPeer.Peers {
			// Check if the peer's NodeName is present in the podMap
			if _, exists := podMap[peer.NodeName]; exists {
				log.Info("Generating config for node:", peer.NodeName)
				BulkTrustePeers(cfg, *mutualPeer)
				break // Skip to the next mutualPeer
			}
		}
	}

	return nodeIDsMap, nil
}

func BulkTrustePeers(cfg config.MutualPeersConfig, pods config.MutualPeer) {
	// Get the data from the map
	data := GetAllIDs()

	// Loop through the peers in the config and check if they have the TP-ADDR file
	for key := range data {
		log.Info("Data key: ", key)
		for index, pod := range pods.Peers {
			log.Info("INDEX: ", index)
			if key != pod.NodeName {
				log.Info("this is a different one: ", data[key], " ", pod.NodeName)
				command := BulkTrustedPeerCommand(data[key])
				output, err := RunRemoteCommand(
					pod.NodeName,
					pod.ContainerName,
					GetCurrentNamespace(),
					command)
				if err != nil {
					log.Error("Error executing remote command: ", err)
				}
				log.Info("OUTPUT: ", output)
			}
		}
	}
}

// RunRemoteCommand executes a remote command on the specified node.
func RunRemoteCommand(nodeName, container, namespace string, command []string) (string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error("Error: ", err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error("Error: ", err.Error())
		panic(err.Error())
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
	output, err := executeCommand(config, req)
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
