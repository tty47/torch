package k8s

import (
	"context"
	"strings"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/jrmanes/torch/pkg/db/redis"
)

const queueK8SNodes = "k8s"

// WatchStatefulSets watches for changes to the StatefulSets in the specified namespace and updates the metrics accordingly
func WatchStatefulSets() error {
	// namespace get the current namespace where torch is running
	namespace := GetCurrentNamespace()
	// Authentication in cluster - using Service Account, Role, RoleBinding
	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Error("Error: ", err)
		return err
	}

	// Create the Kubernetes clientSet
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error("Error: ", err)
		return err
	}

	// Create a StatefulSet watcher
	watcher, err := clientSet.AppsV1().StatefulSets(namespace).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Error("Error: ", err)
		return err
	}

	// Watch for events on the watcher channel
	for event := range watcher.ResultChan() {
		// Check if the event object is of type *v1.StatefulSet
		if statefulSet, ok := event.Object.(*v1.StatefulSet); ok {
			// Check if the StatefulSet name has the "da" prefix
			if strings.HasPrefix(statefulSet.Name, "da") {
				// Check if the StatefulSet is in the "Running" state
				if statefulSet.Status.CurrentReplicas > 0 &&
					statefulSet.Status.Replicas == statefulSet.Status.ReadyReplicas {
					// Perform necessary actions, such as adding the node to the Redis queue
					err := redis.Producer(statefulSet.Name, queueK8SNodes)
					if err != nil {
						log.Error("ERROR adding the node to the queue: ", err)
						return err
					}
				}
			}
		}
	}

	return nil
}
