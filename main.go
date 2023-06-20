package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

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

// MutualPeersConfig represents the configuration structure.
type MutualPeersConfig struct {
	// List of mutual peers.
	MutualPeers []*MutualPeer `yaml:"mutualPeers"`
}

// MutualPeer represents a mutual peer structure.
type MutualPeer struct {
	// List of peers.
	Peers []Peer `yaml:"peers"`
}

// Peer represents a peer structure.
type Peer struct {
	// NodeName of the peer node.
	NodeName string `yaml:"nodeName"`
}

// Configuration variables
var (
	// cfg stores the mutual peers configuration.
	cfg MutualPeersConfig
	// currentNamespace Stores the current namespace.
	currentNamespace string
)

// ParseFlags parses the command-line flags and reads the configuration file.
func ParseFlags() MutualPeersConfig {
	// Define the flag for the configuration file path
	configFile := flag.String("config-file", "", "Path to the configuration file")

	// Parse the flags
	flag.Parse()

	// Read the configuration file
	file, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Error("Config file doesn't exist...", err)
		panic(err)
	}

	// Unmarshal the YAML into a struct
	err = yaml.Unmarshal(file, &cfg)
	if err != nil {
		log.Error("Cannot unmarshal the config file...", err)
		panic(err)
	}

	return cfg
}

// GetConfig handles the HTTP GET request for retrieving the config as JSON.
func GetConfig(w http.ResponseWriter, r *http.Request) {
	// Generate the response, including the configuration
	resp := Response{
		Status: http.StatusOK,
		Body:   cfg,
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

// List handles the HTTP GET request for retrieving the list of matching pods as JSON.
func List(w http.ResponseWriter, r *http.Request) {
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

	log.Info("Namespace: ", currentNamespace)

	// get pods in the current namespace
	pods, err := clientset.CoreV1().Pods(currentNamespace).List(context.TODO(), metav1.ListOptions{})
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

	// Generate the response, adding the matching pod names
	resp := Response{
		Status: http.StatusOK,
		Body:   matchingPods,
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

func GenerateConfig(w http.ResponseWriter, r *http.Request) {
	var body RequestBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		log.Error("Error decoding the request body into the struct:", err)
	}

	log.Info(body.Body)
	//command := []string{"echo", "Hello, World"}
	command := []string{"touch", "/tmp/created-by-mp-orch"}

	err = RunRemoteCommand("da-bridge-0", "da", "default", command)
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

	// Convert the command slice to a single string value
	//commandStr := strings.Join(command, " ")

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

// logRequest is a middleware function that logs the incoming request.
func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info(r.Method, " ", r.URL.Path)
		handler.ServeHTTP(w, r)
	})
}

func init() {
	currentNamespace = os.Getenv("POD_NAMESPACE")
	if currentNamespace == "" {
		log.Info("Current Namespace variable is not defined, using the default value")
		currentNamespace = "default"
	}
}

func main() {
	// Parse the command-line flags and read the configuration file
	log.Info("Running on namespace: ", currentNamespace)
	cfg = ParseFlags()
	log.Info("Config File:\n", cfg)

	httpPort := "8080"

	// Set up the HTTP server
	r := mux.NewRouter()
	r.Use(logRequest)
	r.HandleFunc("/config", GetConfig).Methods("GET")
	r.HandleFunc("/list", List).Methods("GET")
	r.HandleFunc("/gen", GenerateConfig).Methods("POST")

	server := &http.Server{
		Addr:    ":" + httpPort,
		Handler: r,
	}

	// Start the HTTP server
	log.Info("Server started on port ", httpPort)
	log.Fatal(server.ListenAndServe())
}
