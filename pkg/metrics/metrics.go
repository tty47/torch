package metrics

import (
	"context"

	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Get the meter from the global meter provider with the name "torch".
var meter = otel.GetMeterProvider().Meter("torch")

// MultiAddrs represents the information for a multiaddress.
type MultiAddrs struct {
	ServiceName string  // ServiceName Name of the service associated with the multiaddress.
	NodeName    string  // NodeName Name of the node.
	MultiAddr   string  // MultiAddr Multiaddress value.
	Namespace   string  // Namespace where the service is deployed.
	Value       float64 // Value to be observed for the multiaddress.
}

// WithMetricsMultiAddress creates a callback function to observe metrics for multiple multiaddresses.
func WithMetricsMultiAddress(multiAddrs []MultiAddrs) error {
	log.Info("registering metric: ", multiAddrs)
	// Create a Float64ObservableGauge named "multiaddress" with a description for the metric.
	multiAddressesGauge, err := meter.Float64ObservableGauge(
		"multiaddr",
		metric.WithDescription("Torch - MultiAddresses"),
	)
	if err != nil {
		log.Fatalf(err.Error())
		return err
	}

	// Define the callback function that will be called periodically to observe metrics.
	callback := func(ctx context.Context, observer metric.Observer) error {
		for _, ma := range multiAddrs {
			// Create labels with attributes for each multiaddress.
			labels := metric.WithAttributes(
				attribute.String("service_name", ma.ServiceName),
				attribute.String("node_name", ma.NodeName),
				attribute.String("multiaddress", ma.MultiAddr),
				attribute.String("namespace", ma.Namespace),
			)
			// Observe the float64 value for the current multiaddress with the associated labels.
			observer.ObserveFloat64(multiAddressesGauge, ma.Value, labels)
		}

		return nil
	}

	// Register the callback with the meter and the Float64ObservableGauge.
	_, err = meter.RegisterCallback(callback, multiAddressesGauge)
	return err
}

// BlockHeight represents the information for the block height 1.
type BlockHeight struct {
	ServiceName string  // ServiceName Name of the service associated with the multiaddress.
	BlockHeight string  // Namespace where the service is deployed.
	Value       float64 // Value to be observed for the multiaddress.
}

// WithMetricsBlockHeight creates a callback function to observe metrics for block_height_1.
// consensus-node:26657/block?height=1
func WithMetricsBlockHeight(blockHeight, earliestBlockTime, serviceName, namespace string) error {
	log.Info("registering metric: ", blockHeight)
	// Create a Float64ObservableGauge named "block_height_1" with a description for the metric.
	blockHeightGauge, err := meter.Float64ObservableGauge(
		"block_height_1",
		metric.WithDescription("Torch - BlockHeight"),
	)
	if err != nil {
		log.Fatalf(err.Error())
		return err
	}
	callback := func(ctx context.Context, observer metric.Observer) error {
		// Define the callback function that will be called periodically to observe metrics.
		// Create labels with attributes for each block_height_1.
		labels := metric.WithAttributes(
			attribute.String("service_name", serviceName),
			attribute.String("block_height_1", blockHeight),
			attribute.String("earliest_block_time", earliestBlockTime),
			attribute.String("namespace", namespace),
		)
		// Observe the float64 value for the current block_height_1 with the associated labels.
		observer.ObserveFloat64(blockHeightGauge, 1, labels)

		return nil
	}

	// Register the callback with the meter and the Float64ObservableGauge.
	_, err = meter.RegisterCallback(callback, blockHeightGauge)
	return err
}
