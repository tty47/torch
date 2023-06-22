package main

import (
	"flag"
	"io/ioutil"

	"github.com/jrmanes/torch/config"
	handlers "github.com/jrmanes/torch/pkg/http"

	"github.com/jrmanes/torch/pkg/k8s"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

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

	handlers.Run(cfg)
}
