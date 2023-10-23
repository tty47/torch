package metrics

import (
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// InitConfig initializes the configs Prometheus - OTEL
func InitConfig() error {
	// Initialize the Prometheus exporter
	log.Info("Initializing Prometheus client...")
	exporter, err := prometheus.New()
	if err != nil {
		log.Fatal(err)
		return err
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))

	log.Info("Initializing OTEL Provider...")
	otel.SetMeterProvider(provider)

	return nil
}
