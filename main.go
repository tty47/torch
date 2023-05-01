package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Response struct {
	Status int         `json:"status"`
	Body   interface{} `json:"body"`
	Errors interface{} `json:"errors,omitempty"`
}

type Namespace struct {
	Namespace string `json:"namespace"`
	Nodes     []Node `json:"nodes"`
}

type Node struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Multiaddress string `json:"multiaddress"`
}

type MutualPeersConfig struct {
	MutualPeers []PeersConfig `yaml:"mutualPeers"`
}

type PeersConfig struct {
	Peers []string `yaml:"peers"`
}

func namespaceHandler(w http.ResponseWriter, r *http.Request) {
	// Log the request details
	reqLog := map[string]string{
		"remote_addr": r.RemoteAddr,
		"method":      r.Method,
		"url":         r.URL.Path,
	}

	reqLogJSON, _ := json.Marshal(reqLog)
	log.Printf(string(reqLogJSON))

	if r.URL.Path != "/" {
		http.NotFound(w, r)
		errorLog := map[string]interface{}{
			"status":  404,
			"message": "Not Found",
		}
		handleError(w, http.StatusNotFound, errorLog)
		return
	}

	ns := getNamespace()
	res := Response{
		Status: http.StatusOK,
		Body: struct {
			Namespace string `json:"namespace"`
			Nodes     []Node `json:"nodes"`
		}{
			Namespace: ns.Namespace,
			Nodes:     ns.Nodes,
		},
		Errors: 0,
	}

	resJSON, _ := json.Marshal(res)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resJSON)

	// Log the response details
	log.Printf(string(resJSON))
}

func listPodsHandler(w http.ResponseWriter, r *http.Request) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	// get pods in all the namespaces by omitting namespace
	// Or specify namespace to get pods in particular namespace
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	//		pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), v1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

	// Print the name of each pod
	for _, pod := range pods.Items {
		fmt.Printf("Pod: %s\n", pod.Name)
	}

	res := Response{
		Status: http.StatusOK,
		Body:   pods.Items,
		Errors: 0,
	}

	resJSON, _ := json.Marshal(res)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resJSON)

	// Log the response details
	//log.Printf(string(resJSON))
	// Examples for error handling:
	// - Use helper functions e.g. errors.IsNotFound()
	// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
	// _, err = clientset.CoreV1().Pods("default").Get(context.TODO(), "example-xxxxx", metav1.GetOptions{})
	// if errors.IsNotFound(err) {
	// 	fmt.Printf("Pod example-xxxxx not found in default namespace\n")
	// } else if statusError, isStatus := err.(*errors.StatusError); isStatus {
	// 	fmt.Printf("Error getting pod %v\n", statusError.ErrStatus.Message)
	// } else if err != nil {
	// 	panic(err.Error())
	// } else {
	// 	fmt.Printf("Found example-xxxxx pod in default namespace\n")
	// }

	// time.Sleep(10 * time.Second)
}

func getNamespace() Namespace {
	return Namespace{
		Namespace: "example",
		Nodes: []Node{
			{Name: "node1", Type: "type1", Multiaddress: "/ip4/127.0.0.1/tcp/5001"},
			{Name: "node2", Type: "type2", Multiaddress: "/ip4/127.0.0.1/tcp/5002"},
			{Name: "node3", Type: "type3", Multiaddress: "/ip4/127.0.0.1/tcp/5003"},
			{Name: "node4", Type: "type4", Multiaddress: "/ip4/127.0.0.1/tcp/5004"},
		},
	}
}

func handleError(w http.ResponseWriter, status int, errorLog interface{}) {
	errorLogJSON, _ := json.Marshal(errorLog)
	log.Printf(string(errorLogJSON))

	res := Response{
		Status: status,
		Body:   nil,
		Errors: errorLog,
	}

	resJSON, _ := json.Marshal(res)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(resJSON)
}

func ParseFlags() {
	// Define the flag
	configFile := flag.String("config-file", "", "Path to the configuration file")

	// Parse the flags
	flag.Parse()

	// Read the file
	file, err := ioutil.ReadFile(*configFile)
	if err != nil {
		panic(err)
	}

	// Unmarshal the YAML into a struct
	var config MutualPeersConfig
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		panic(err)
	}

	// Print the struct
	fmt.Printf("Config File: %+v\n", config)
}

func main() {
	ParseFlags()

	router := http.NewServeMux()
	router.HandleFunc("/", namespaceHandler)
	router.HandleFunc("/pods", listPodsHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	log.Printf("Server started on port 8080")
	log.Fatal(server.ListenAndServe())
}
