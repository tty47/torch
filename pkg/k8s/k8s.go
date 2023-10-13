package k8s

import (
	"bytes"
	"os"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

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

// RunRemoteCommand executes a remote command on the specified node.
func RunRemoteCommand(nodeName, container, namespace string, command []string) (string, error) {
	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Error("Error: ", err.Error())
	}
	// creates the client
	client, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("Error: %v", err.Error())
	}

	// Create a request to execute the command on the specified node.
	req := client.CoreV1().RESTClient().Post().
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
