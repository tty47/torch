package handlers

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/jrmanes/torch/config"
	"github.com/jrmanes/torch/pkg/db/redis"
	"github.com/jrmanes/torch/pkg/k8s"
	"github.com/jrmanes/torch/pkg/metrics"
	"github.com/jrmanes/torch/pkg/nodes"
)

const (
	retryInterval        = 10 * time.Second // retryInterval Retry interval in seconds to generate the consensus metric.
	hashMetricGenTimeout = 5 * time.Minute  // hashMetricGenTimeout specify the max time to retry to generate the metric.
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

	// check if Torch has to generate the metric or not, we invoke this function async to continue the execution flow.
	go BackgroundGenerateHashMetric(cfg)
	go BackgroundGenerateLBMetric()

	// Initialize the goroutine to check the nodes in the queue.
	log.Info("Initializing queues to process the nodes...")
	// Create a new context without timeout as we want to keep this goroutine running forever, if we specify a timeout,
	// it will be canceled at some point.c
	go func() {
		go nodes.ProcessTaskQueue()
	}()

	log.Info("Initializing goroutine to watch over the StatefulSets...")
	// Initialize a goroutine to watch for changes in StatefulSets in the namespace.
	go func() {
		// Call the WatchStatefulSets function and capture any potential error.
		err := k8s.WatchStatefulSets()
		if err != nil {
			// Log an error message if WatchStatefulSets encounters an error.
			log.Error("Error in WatchStatefulSets: ", err)
		}
	}()

	// Initialize the goroutine to add a watcher to the StatefulSets in the namespace.
	log.Info("Initializing Redis consumer")
	go func() {
		nodes.ConsumerInit("k8s")
	}()

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

// BackgroundGenerateLBMetric initializes a goroutine to generate the load_balancer metric.
func BackgroundGenerateLBMetric() {
	log.Info("Initializing goroutine to generate the metric: load_balancer ")

	// Retrieve the list of Load Balancers
	_, err := k8s.RetrieveAndGenerateMetrics()
	if err != nil {
		log.Printf("Failed to update metrics: %v", err)
	}

	// Start watching for changes to the services in a separate goroutine
	done := make(chan error)
	go k8s.WatchServices(done)

	// Handle errors from WatchServices
	for {
		select {
		case err := <-done:
			if err != nil {
				log.Error("Error in WatchServices: ", err)
			}
		}
	}
}

// BackgroundGenerateHashMetric checks if the consensusNode field is defined in the config to generate the metric from the Genesis Hash data.
func BackgroundGenerateHashMetric(cfg config.MutualPeersConfig) {
	log.Info("BackgroundGenerateHashMetric...")

	if cfg.MutualPeers[0].ConsensusNode != "" {
		log.Info("Initializing goroutine to generate the metric: hash ")

		// Create an errgroup with a context
		eg, ctx := errgroup.WithContext(context.Background())

		// Run the WatchHashMetric function in a separate goroutine
		eg.Go(func() error {
			log.Info("Consensus node defined to get the first block")
			return WatchHashMetric(cfg, ctx)
		})

		// Wait for all goroutines to finish
		if err := eg.Wait(); err != nil {
			log.Error("Error in BackgroundGenerateHashMetric: ", err)
			// Handle the error as needed
		}
	}
}

// WatchHashMetric watches for changes to generate hash metrics in the specified interval.
func WatchHashMetric(cfg config.MutualPeersConfig, ctx context.Context) error {
	// Create a new context derived from the input context with a timeout
	ctx, cancel := context.WithTimeout(ctx, hashMetricGenTimeout)
	defer cancel()

	// Create an errgroup with the context
	eg, ctx := errgroup.WithContext(ctx)

	// Run the WatchHashMetric function in a separate goroutine
	eg.Go(func() error {
		return watchMetricsWithRetry(cfg, ctx)
	})

	// Wait for all goroutines to finish
	return eg.Wait()
}

// watchMetricsWithRetry is a helper function for WatchHashMetric that encapsulates the retry logic.
func watchMetricsWithRetry(cfg config.MutualPeersConfig, ctx context.Context) error {
	// Continue generating metrics with retries
	for {
		select {
		case <-ctx.Done():
			// Context canceled, stop the process
			log.Info("Context canceled, stopping metrics watch process.")
			return ctx.Err()
		default:
			hashMetricsErr := GenerateHashMetrics(cfg)
			consensusMetricsErr := ConsNodesIDs(cfg)

			// Check if both metrics generation are successful
			if hashMetricsErr == nil && consensusMetricsErr == nil {
				log.Info("Metrics generated successfully, stopping the process...")
				return nil
			}

			// Log errors if they occur
			if hashMetricsErr != nil {
				log.Error("Error generating hash metrics: ", hashMetricsErr)
			}
			if consensusMetricsErr != nil {
				log.Error("Error generating consensus node ID metrics: ", consensusMetricsErr)
			}

			// Wait for the retry interval before the next execution using a timer
			if err := waitForRetry(ctx); err != nil {
				return err
			}
		}
	}
}

// waitForRetry is a helper function to wait for the retry interval or stop if the context is canceled.
func waitForRetry(ctx context.Context) error {
	timer := time.NewTimer(retryInterval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		// Context canceled, stop the process
		log.Info("Context canceled, stopping WatchHashMetric.")
		return ctx.Err()
	case <-timer.C:
		// Continue to the next iteration
		return nil
	}
}

// GenerateHashMetrics generates the metric by getting the first block and calculating the days.
func GenerateHashMetrics(cfg config.MutualPeersConfig) error {
	log.Info("Trying to generate the metric for the first block generated...")

	// Get the genesisHash
	blockHash, earliestBlockTime, err := nodes.GenesisHash(cfg.MutualPeers[0].ConsensusNode)
	if err != nil {
		return err
	}

	// check if earliestBlockTime is not empty, otherwise torch skips this process for now.
	err = metrics.WithMetricsBlockHeight(
		blockHash,
		earliestBlockTime,
		cfg.MutualPeers[0].ConsensusNode,
		os.Getenv("POD_NAMESPACE"),
	)
	if err != nil {
		log.Error("Error registering metric block_height_1: ", err)
		return err
	}

	return nil
}

// ConsNodesIDs generates the metric with the consensus nodes ids.
func ConsNodesIDs(cfg config.MutualPeersConfig) error {
	log.Info("Generating the metric for the consensus nodes ids...")

	for _, mutualPeer := range cfg.MutualPeers {
		for _, peer := range mutualPeer.Peers {
			if peer.NodeType == "consensus" {
				consNodeId, err := nodes.ConsensusNodesIDs(peer.ServiceName)
				if err != nil {
					log.Error("Error getting consensus node ID for service [", peer.ServiceName, "]: ", err)
					return err
				}

				err = metrics.RegisterConsensusNodeMetric(
					consNodeId,
					peer.ServiceName,
					os.Getenv("POD_NAMESPACE"),
				)
				if err != nil {
					log.Error("Error registering metric for service [", peer.ServiceName, "]: ", err)
					return err
				}
			}
		}
	}

	return nil
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
