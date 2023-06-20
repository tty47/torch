package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
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
	Label string `yaml:"label"`
}

var config MutualPeersConfig

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
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		log.Error("Cannot unmarshal the config file...", err)
		panic(err)
	}

	return config
}

// GetConfig handles the HTTP GET request for retrieving the config as JSON.
func GetConfig(w http.ResponseWriter, r *http.Request) {
	// Generate the response, adding the size of the array
	resp := Response{
		Status: http.StatusOK,
		Body:   config,
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

func main() {
	// Parse the command-line flags and read the configuration file
	config = ParseFlags()
	log.Info("Config File:\n", config)

	// Access the parsed data
	for _, mutualPeer := range config.MutualPeers {
		log.Info("Peers:")
		for _, peer := range mutualPeer.Peers {
			log.Info("Label:", peer.Label)
		}
	}

	httpPort := "8080"

	// Set up the HTTP server
	r := mux.NewRouter()
	r.Use(logRequest)
	r.HandleFunc("/config", GetConfig).Methods("GET")

	server := &http.Server{
		Addr:    ":" + httpPort,
		Handler: r,
	}

	// Start the HTTP server
	log.Info("Server started on port ", httpPort)
	log.Fatal(server.ListenAndServe())
}
