package metrics

import (
	"context"

	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Get the meter from the global meter provider with the name "torch".
var meter = otel.GetMeterProvider().Meter("multiaddr")

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
