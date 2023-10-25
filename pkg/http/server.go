package handlers

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/celestiaorg/torch/config"
	"github.com/celestiaorg/torch/pkg/metrics"
	"github.com/celestiaorg/torch/pkg/nodes"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// GetHttpPort GetPort retrieves the namespace where the service will be deployed
func GetHttpPort() string {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		log.Info("Using the default port: 8080")
		return "8080"
	}

	// Ensure that the provided port is a valid numeric value
	_, err := strconv.Atoi(port)
	if err != nil {
		log.Error("Invalid HTTP_PORT [", os.Getenv("HTTP_PORT"), "] ,using default port 8080")
		return "8080"
	}

	return port
}

// Run initializes the HTTP server, registers metrics for all nodes in the configuration,
// and starts the server.
func Run(cfg config.MutualPeersConfig) {
	// Get http port
	httpPort := GetHttpPort()

	// Set up the HTTP server
	r := mux.NewRouter()
	// Get the routers
	r = Router(r, cfg)
	// Use the middleware
	r.Use(LogRequest)

	// Initialize the config and register the metrics for all nodes
	err := metrics.InitConfig()
	if err != nil {
		log.Errorf("Error initializing metrics: %v", err)
		return
	}

	// generate the metric from the Genesis Hash data
	notOk := GenerateHashMetrics(cfg, err)
	if notOk {
		log.Error("Error registering metric block_height_1")
		return
	}

	// Create the server
	server := &http.Server{
		Addr:    ":" + httpPort,
		Handler: r,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("Listening on: %v", err)
		}
	}()
	log.Info("Server Started...")
	log.Info("Listening on port: " + httpPort)

	log.Info("Initializing queues to process the nodes...")
	go nodes.ProcessTaskQueue()

	<-done
	log.Info("Server Stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	if err := server.Shutdown(ctx); err != nil {
		log.Errorf("Server Shutdown Failed: %v", err)
	}
	log.Info("Server Exited Properly")
}

func GenerateHashMetrics(cfg config.MutualPeersConfig, err error) bool {
	// Get the genesisHash
	// check if the config has the consensusNode field defined
	if cfg.MutualPeers[0].ConsensusNode != "" {
		blockHash, earliestBlockTime := nodes.GenesisHash(cfg)
		err = metrics.WithMetricsBlockHeight(
			blockHash,
			earliestBlockTime,
			cfg.MutualPeers[0].ConsensusNode,
			os.Getenv("POD_NAMESPACE"),
		)
		if err != nil {
			log.Errorf("Error registering metric block_height_1: %v", err)
			return true
		}
	}
	return false
}

//
//// RegisterMetrics generates and registers the metrics for all nodes in the configuration.
//func RegisterMetrics(cfg config.MutualPeersConfig) error {
//	log.Info("Generating initial metrics for all the nodes...")
//
//	var nodeNames []string
//
//	// Adding nodes from config to register the initial metrics
//	for _, n := range cfg.MutualPeers {
//		for _, no := range n.Peers {
//			nodeNames = append(nodeNames, no.NodeName)
//		}
//	}
//
//	// Generate the metrics for all nodes
//	_, err := nodes.GenerateAllTrustedPeersAddr(cfg, nodeNames)
//	if err != nil {
//		log.Errorf("Error GenerateAllTrustedPeersAddr: %v", err)
//		return err
//	}
//
//	return nil
//}
