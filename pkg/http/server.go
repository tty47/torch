package handlers

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jrmanes/torch/config"
	"github.com/jrmanes/torch/pkg/db/redis"
	"github.com/jrmanes/torch/pkg/k8s"
	"github.com/jrmanes/torch/pkg/metrics"
	"github.com/jrmanes/torch/pkg/nodes"

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

	// Initialize the goroutine to check the nodes in the queue.
	log.Info("Initializing queues to process the nodes...")
	go nodes.ProcessTaskQueue()

	// Initialize the goroutine to add a watcher to the StatefulSets in the namespace.
	log.Info("Initializing goroutine to watch over the StatefulSets...")
	go k8s.WatchStatefulSets()

	// Initialize the goroutine to add a watcher to the StatefulSets in the namespace.
	log.Info("Initializing Redis consumer")
	go nodes.ConsumerInit("k8s")

	// Check if we already have some multi addresses in the DB and expose them, there might be a situation where Torch
	// get restarted, and we already have the nodes IDs, so we can expose them.
	err = RegisterMetrics(cfg)
	if err != nil {
		log.Error("Couldn't generate the metrics...", err)
	}

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

// RegisterMetrics generates and registers the metrics for all nodes in case they already exist in the DB.
func RegisterMetrics(cfg config.MutualPeersConfig) error {
	red := redis.InitRedisConfig()
	// Create a new context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)

	// Make sure to call the cancel function to release resources when you're done
	defer cancel()

	log.Info("Generating metrics from existing nodes...")

	// Adding nodes from config to register the initial metrics
	for _, n := range cfg.MutualPeers {
		for _, no := range n.Peers {
			// checking the node in the DB first
			ma, err := redis.CheckIfNodeExistsInDB(red, ctx, no.NodeName)
			if err != nil {
				log.Error("Error CheckIfNodeExistsInDB : [", no.NodeName, "]", err)
				return err
			}

			// check if the multi address is not empty
			if ma != "" {
				log.Info("Node: [", no.NodeName, "], found in the DB generating metric: ", " [", ma, "]")

				// Register a multi-address metric
				m := metrics.MultiAddrs{
					ServiceName: "torch",
					NodeName:    no.NodeName,
					MultiAddr:   ma,
					Namespace:   no.Namespace,
					Value:       1,
				}
				metrics.RegisterMetric(m)
			}
		}
	}

	return nil
}
