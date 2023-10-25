package k8s

import (
	"context"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// WatchStatefulSets watches for changes to the StatefulSets in the specified namespace and updates the metrics accordingly
func WatchStatefulSets() {
	namespace := GetCurrentNamespace()
	// Authentication in cluster - using Service Account, Role, RoleBinding
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
		return
	}

	// Create the Kubernetes clientSet
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
		return
	}

	// Create a StatefulSet watcher
	watcher, err := clientSet.AppsV1().StatefulSets(namespace).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Fatal(err)
		return
	}

	// Watch for events on the watcher channel
	for event := range watcher.ResultChan() {
		if statefulSet, ok := event.Object.(*v1.StatefulSet); ok {
			log.Info("StatefulSet: ", statefulSet)
			log.Info("StatefulSet name: ", statefulSet.Name)
			log.Info("StatefulSet namespace: ", statefulSet.Namespace)
			log.Info("StatefulSet containers: ", statefulSet.Spec.Template.Spec.Containers)
		}
	}
}
