package main

import (
	"flag"
	"io/ioutil"
	"net/http"

	"github.com/jrmanes/mp-orch/config"
	handlers "github.com/jrmanes/mp-orch/pkg/http"
	"github.com/jrmanes/mp-orch/pkg/k8s"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
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

// Configuration variables
var (
	// cfg stores the mutual peers configuration.
	cfg config.MutualPeersConfig
)

// ParseFlags parses the command-line flags and reads the configuration file.
func ParseFlags() config.MutualPeersConfig {
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

func main() {
	// Parse the command-line flags and read the configuration file
	log.Info("Running on namespace: ", k8s.GetCurrentNamespace())
	cfg = ParseFlags()
	log.Info("Config File:\n", cfg)

	httpPort := "8080"

	// Set up the HTTP server
	r := mux.NewRouter()
	r.Use(handlers.LogRequest)
	r.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		handlers.GetConfig(w, r, cfg)
	}).Methods("GET")
	r.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		handlers.List(w, r, cfg)
	}).Methods("GET")
	r.HandleFunc("/gen", func(w http.ResponseWriter, r *http.Request) {
		k8s.GenerateTrustedPeersAddr(w, r, cfg)
	}).Methods("POST")

	server := &http.Server{
		Addr:    ":" + httpPort,
		Handler: r,
	}

	// Start the HTTP server
	log.Info("Server started on port ", httpPort)
	log.Fatal(server.ListenAndServe())
}
