package k8s

import (
	"github.com/celestiaorg/torch/pkg/metrics"

	log "github.com/sirupsen/logrus"
)

// Declare a slice to hold multiple MultiAddrs metrics.
var multiAddresses []metrics.MultiAddrs

// MultiAddrExists checks if a given MultiAddr already exists in the multiAddresses slice.
// It returns true if the MultiAddr already exists, and false otherwise.
func MultiAddrExists(multiAddr string) bool {
	for _, addr := range multiAddresses {
		// Compare each MultiAddr in the slice with the provided multiAddr.
		if addr.MultiAddr == multiAddr {
			return true
		}
	}
	return false
}

// RegisterMetric adds a new MultiAddrs metric to the multiAddresses slice.
// Before adding, it checks if the MultiAddr already exists in the slice using MultiAddrExists function.
// If the MultiAddr already exists, it logs a message and skips the addition.
// Otherwise, it appends the new MultiAddrs to the slice and registers the updated metrics.
func RegisterMetric(m metrics.MultiAddrs) {
	// Check if the MultiAddr already exists in the array
	if MultiAddrExists(m.MultiAddr) {
		log.Info("MultiAddr already exists in the metrics array: ", m.NodeName, " ", m.MultiAddr)
		return
	}

	// Append the new MultiAddr to the array
	multiAddresses = append(multiAddresses, m)

	// Register the metric
	err := metrics.WithMetricsMultiAddress(multiAddresses)
	if err != nil {
		log.Printf("Failed to update metrics: %v", err)
	}
}
