package k8s

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/celestiaorg/torch/pkg/metrics"
)

// RetrieveAndGenerateMetrics retrieves the list of Load Balancers and generates metrics
func RetrieveAndGenerateMetrics() ([]metrics.LoadBalancer, error) {
	log.Info("Retrieving the list of Load Balancers")

	// Get list of LBs
	svc, err := ListServices()
	if err != nil {
		log.Error("Failed to retrieve the LoadBalancers: ", err)
		return nil, err
	}

	// Get the list of the LBs
	loadBalancers, err := GetLoadBalancers(svc)
	if err != nil {
		log.Error("Error getting the load balancers: ", err)
		return nil, err
	}

	// Generate the metrics with the LBs
	err = metrics.WithMetricsLoadBalancer(loadBalancers)
	if err != nil {
		log.Error("Failed to update metrics: ", err)
		return nil, err
	}

	return loadBalancers, nil
}

// ListServices retrieves the list of services in a namespace
func ListServices() (*corev1.ServiceList, error) {
	// Authentication in cluster - using Service Account, Role, RoleBinding
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error("ERROR: ", err)
		return nil, err
	}

	// Create the Kubernetes clientSet
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error("ERROR: ", err)
		return nil, err
	}

	// Get all services in the namespace
	services, err := clientSet.CoreV1().Services(GetCurrentNamespace()).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Error("ERROR: ", err)
		return nil, err
	}

	return services, nil
}

// GetLoadBalancers filters the list of services to include only Load Balancers and returns a list of them
func GetLoadBalancers(svc *corev1.ServiceList) ([]metrics.LoadBalancer, error) {
	var loadBalancers []metrics.LoadBalancer

	for _, svc := range svc.Items {
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			for _, ingress := range svc.Status.LoadBalancer.Ingress {
				log.Info(fmt.Sprintf("Updating metrics for service: [%s] with IP: [%s]", svc.Name, ingress.IP))

				// Create a LoadBalancer struct and append it to the loadBalancers list
				loadBalancer := metrics.LoadBalancer{
					ServiceName:      "torch",
					LoadBalancerName: svc.Name,
					LoadBalancerIP:   ingress.IP,
					Namespace:        svc.Namespace,
					Value:            1, // Set the value of the metric here (e.g., 1)
				}
				loadBalancers = append(loadBalancers, loadBalancer)
			}
		}
	}

	if len(loadBalancers) == 0 {
		return nil, errors.New("no Load Balancers found")
	}

	return loadBalancers, nil
}

// WatchServices watches for changes to the services in the specified namespace and updates the metrics accordingly
func WatchServices(done chan<- error) {
	defer close(done)

	// Authentication in cluster - using Service Account, Role, RoleBinding
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error("Failed to get in-cluster config: ", err)
		done <- err
		return
	}

	// Create the Kubernetes clientSet
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error("Failed to create Kubernetes clientSet: ", err)
		done <- err
		return
	}

	// Create a service watcher
	watcher, err := clientSet.CoreV1().Services(GetCurrentNamespace()).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Error("Failed to create service watcher: ", err)
		done <- err
		return
	}

	// Watch for events on the watcher channel
	for event := range watcher.ResultChan() {
		if service, ok := event.Object.(*corev1.Service); ok {
			if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
				loadBalancers, err := GetLoadBalancers(&corev1.ServiceList{Items: []corev1.Service{*service}})
				if err != nil {
					log.Error("Failed to get the load balancers metrics: %v", err)
					done <- err
					return
				}

				if err := metrics.WithMetricsLoadBalancer(loadBalancers); err != nil {
					log.Error("Failed to update metrics with load balancers: ", err)
					done <- err
					return
				}
			}
		}
	}
}
