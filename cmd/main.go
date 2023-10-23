package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/celestiaorg/torch/config"
	handlers "github.com/celestiaorg/torch/pkg/http"
	"github.com/celestiaorg/torch/pkg/k8s"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Configuration variables
var (
	cfg config.MutualPeersConfig // cfg stores the mutual peers' configuration.
)

// ParseFlags parses the command-line flags and reads the configuration file.
func ParseFlags() config.MutualPeersConfig {
	// Define the flag for the configuration file path
	configFile := flag.String("config-file", "", "Path to the configuration file")

	// Parse the flags
	flag.Parse()

	// Read the configuration file
	file, err := os.ReadFile(*configFile)
	if err != nil {
		log.Error("Config file doesn't exist...", err)
	}

	// Unmarshal the YAML into a struct
	err = yaml.Unmarshal(file, &cfg)
	if err != nil {
		log.Error("Cannot unmarshal the config file...", err)
	}

	return cfg
}

func PrintName() {
	torch := `
================================ 
  __________  ____  ________  __
 /_  __/ __ \/ __ \/ ____/ / / /
  / / / / / / /_/ / /   / /_/ / 
 / / / /_/ / _, _/ /___/ __  /  
/_/  \____/_/ |_|\____/_/ /_/
================================
`
	fmt.Println(torch)
}

func main() {
	PrintName()
	// Parse the command-line flags and read the configuration file
	log.Info("Running on namespace: ", k8s.GetCurrentNamespace())
	cfg = ParseFlags()

	handlers.Run(cfg)
}
