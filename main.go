package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Response represents the response structure.
type Response struct {
	Status int         `json:"status"`
	Body   interface{} `json:"body"`
	Errors interface{} `json:"errors,omitempty"`
}

// MutualPeersConfig represents the configuration structure.
type MutualPeersConfig struct {
	MutualPeers []*MutualPeer `yaml:"mutualPeers"`
}

// MutualPeer represents a mutual peer structure.
type MutualPeer struct {
	Peers []Peer `yaml:"peers"`
}

// Peer represents a peer structure.
type Peer struct {
	NodeName string `yaml:"nodeName"`
}

var (
	cfg              MutualPeersConfig
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
func List(w http.ResponseWriter, r *http.Request) {
	log.Info("------------------------")

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

	// get pods in all the namespaces by omitting namespace
	pods, err := clientset.CoreV1().Pods(currentNamespace).List(context.TODO(), metav1.ListOptions{})
	//pods, err := clientset.CoreV1().Pods(namespace.Items[0].Name).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error("Failed to get pod:", err)
	}

	log.Info("There are ", len(pods.Items), " pods in the cluster")

	// Check if the pods name of the nodeName match the config
	for _, pod := range pods.Items {
		log.Info("================================")
		podName := pod.Name
		log.Info("PodName", podName)
		log.Info("================================")

		for _, mutualPeer := range cfg.MutualPeers {
			for _, peer := range mutualPeer.Peers {
				log.Info("================================")
				log.Info("NodeName", peer.NodeName)
				log.Info("================================")
				if podName == peer.NodeName {
					// Match found
					log.Info("Pod matches the name", pod.Name, peer.NodeName)
					//fmt.Printf("Pod %s matches label %s\n", pod.Name, peer.Label)
				} else {
					log.Info("Pod NOT matches the name", pod.Name, peer.NodeName)
				}
			}
		}
	}

	log.Info("================================")

	// Generate the response, adding the size of the array
	resp := Response{
		Status: http.StatusOK,
		Body:   pods.Items,
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

// GetConfig handles the HTTP GET request for retrieving the config as JSON.
func GetConfig(w http.ResponseWriter, r *http.Request) {
	// Generate the response, adding the size of the array
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

	// Access the parsed data
	for _, mutualPeer := range cfg.MutualPeers {
		log.Info("Peers:")
		for _, peer := range mutualPeer.Peers {
			log.Info("NodeName:", peer.NodeName)
		}
	}

	httpPort := "8080"

	// Set up the HTTP server
	r := mux.NewRouter()
	r.Use(logRequest)
	r.HandleFunc("/config", GetConfig).Methods("GET")
	r.HandleFunc("/list", List).Methods("GET")

	server := &http.Server{
		Addr:    ":" + httpPort,
		Handler: r,
	}

	// Start the HTTP server
	log.Info("Server started on port ", httpPort)
	log.Fatal(server.ListenAndServe())
}
