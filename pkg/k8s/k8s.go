package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type RequestBody struct {
	// Body response response body.
	Body string `json:"podName"`
}

// Response represents the response structure.
type Response struct {
	// Status HTTP code of the response.
	Status int `json:"status"`
	// Body response response body.
	Body interface{} `json:"body"`
	// Errors that occurred during the request, if any.
	Errors interface{} `json:"errors,omitempty"`
}

func GetCurrentNamespace() string {
	// currentNamespace Stores the current namespace.
	currentNamespace := os.Getenv("POD_NAMESPACE")
	if currentNamespace == "" {
		log.Info("Current Namespace variable is not defined, using the default value")
		return "default"
	}
	return currentNamespace
}

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
					// Match found
					log.Info("Pod matches the name: ", pod.Name, " ", peer.NodeName)
					matchingPods = append(matchingPods, podName)
				}
			}
		}
	}

	// matchingPods Stores the matching pods.
	return matchingPods
}

func GetTrustedPeerCommand() []string {
	script := fmt.Sprintf(`#!/bin/sh
# add the prefix to the addr
if [ -f /tmp/TP-ADDR ];then
  cat /tmp/TP-ADDR
fi
`)

	return []string{"sh", "-c", script}
}

func CreateTrustedPeerCommand(cfg config.MutualPeersConfig) []string {
	trusteedPeerPrefix := "/dns/$(hostname)/tcp/2121/p2p/"

	script := fmt.Sprintf(`#!/bin/sh
# add the prefix to the addr
echo -n "%s" > /tmp/TP-ADDR

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

echo -n "${TP_ADDR}" >> /tmp/TP-ADDR
cat /tmp/TP-ADDR
`, trusteedPeerPrefix)

	return []string{"sh", "-c", script}
}

// validateNode check if an input node is available in the config
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

func GenerateTrustedPeersAddr(w http.ResponseWriter, r *http.Request, cfg config.MutualPeersConfig) {
	var body RequestBody

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		log.Error("Error decoding the request body into the struct:", err)
	}

	log.Info(body.Body)
	// TODO: add validation, if the pod is empty, that means that we cannot
	// execute the command, and we have to stop the process here.

	// get the command
	command := CreateTrustedPeerCommand(cfg)

	// validate if the node received is ok
	ok, pod, cont := validateNode(body.Body, cfg)
	if !ok {
		log.Error("Pod name not valid", pod)
		return
	}

	log.Info("Pod found: ", pod, " ", cont, " ", GetCurrentNamespace())

	err = RunRemoteCommand(
		pod,
		cont,
		GetCurrentNamespace(),
		command)
	if err != nil {
		log.Error("Error executing remote command: ", err)
		return
	}

	// Generate the response, adding the matching pod names
	resp := Response{
		Status: http.StatusOK,
		Body:   body,
		Errors: nil,
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		log.Error("Error marshaling to JSON:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonData)
	if err != nil {
		log.Error("Error writing response:", err)
	}
}

// RunRemoteCommand executes a remote command on the specified node.
func RunRemoteCommand(nodeName, container, namespace string, command []string) error {
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

	// Set up the writer for capturing the command output.
	outputWriter := io.Writer(os.Stdout)

	// Execute the remote command.
	err = executeCommand(config, req, outputWriter)
	if err != nil {
		log.Error("failed to execute remote command: ", err)
	}

	return nil
}

// executeCommand executes the remote command using the provided configuration, request, and output writer.
func executeCommand(config *rest.Config, req *rest.Request, outputWriter io.Writer) error {
	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		log.Error("failed to create SPDY executor: ", err)
	}

	// Prepare the standard I/O streams.
	stdin := io.Reader(nil) // Set stdin to nil
	stdout := outputWriter
	stderr := outputWriter

	// Execute the remote command and capture the output.
	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})

	if err != nil {
		log.Error("failed to execute command stream: ", err)
	}

	return nil
}
