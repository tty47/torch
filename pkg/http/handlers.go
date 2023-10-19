package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jrmanes/torch/config"
	"github.com/jrmanes/torch/pkg/db/redis"
	"github.com/jrmanes/torch/pkg/nodes"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type RequestBody struct {
	// Body response response body.
	Body string `json:"pod_name"`
}

type RequestMultipleNodesBody struct {
	// Body response response body.
	Body []string `json:"pod_name"`
}

// Response represents the response structure.
type Response struct {
	// Status HTTP code of the response.
	Status int `json:"status"`
	// Body response response body.
	Body interface{} `json:"body"`
	// Errors that occurred during the request, if any.
	Errors interface{} `json:"errors,omitempty"`
}

// MutualPeersConfig represents the configuration structure.
type MutualPeersConfig struct {
	// List of mutual peers.
	MutualPeers []*MutualPeer `yaml:"mutualPeers"`
}

// MutualPeer represents a mutual peer structure.
type MutualPeer struct {
	// List of peers.
	Peers []Peer `yaml:"peers"`
}

// Peer represents a peer structure.
type Peer struct {
	// NodeName of the peer node.
	NodeName      string `yaml:"nodeName"`
	ContainerName string `yaml:"containerName"`
}

// GetConfig handles the HTTP GET request for retrieving the config as JSON.
func GetConfig(w http.ResponseWriter, r *http.Request, cfg config.MutualPeersConfig) {
	// Generate the response, including the configuration
	resp := Response{
		Status: http.StatusOK,
		Body:   cfg,
		Errors: nil,
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		log.Error("Error marshaling to JSON:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonData)
	if err != nil {
		log.Error("Error writing response:", err)
	}
}

// List handles the HTTP GET request for retrieving the list of matching pods as JSON.
func List(w http.ResponseWriter, r *http.Request, cfg config.MutualPeersConfig) {
	red := redis.InitRedisConfig()
	ctx := context.TODO()

	// get all values from redis
	nodeIDs, err := red.GetAllKeys(ctx)
	if err != nil {
		log.Error("Error getting the keys and values: ", err)
	}

	// Generate the response, adding the matching pod names
	resp := Response{
		Status: http.StatusOK,
		Body:   nodeIDs,
		Errors: nil,
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		log.Error("Error marshaling to JSON:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonData)
	if err != nil {
		log.Error("Error writing response:", err)
	}
}

// GetNoId handles the HTTP GET request for retrieving the list of matching pods as JSON.
func GetNoId(w http.ResponseWriter, r *http.Request, cfg config.MutualPeersConfig) {
	nodeName := mux.Vars(r)["nodeName"]
	if nodeName == "" {
		log.Error("User param nodeName is empty", http.StatusNotFound)
		return
	}

	// verify that the node is in the config
	ok, peer := nodes.ValidateNode(nodeName, cfg)
	if !ok {
		log.Error("Error: Pod doesn't exists in the config")
		resp := Response{
			Status: http.StatusNotFound,
			Body:   peer.NodeName,
			Errors: errors.New("error: Pod doesn't exists in the config"),
		}
		ReturnResponse(resp, w, r)
	}

	red := redis.InitRedisConfig()
	ctx := context.TODO()

	// initialize the response struct
	resp := Response{}

	nodeIDs, err := red.GetKey(ctx, nodeName)
	if err != nil {
		log.Error("Error getting the keys and values: ", err)
	}

	if nodeIDs == "" {
		resp = Response{
			Status: http.StatusNotFound,
			Body:   "",
			Errors: "[ERROR] Node [" + nodeName + "] not found",
		}
	} else {
		// Generate the response, adding the matching pod names
		resp = Response{
			Status: http.StatusOK,
			Body:   nodeIDs,
			Errors: nil,
		}
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		log.Error("Error marshaling to JSON:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonData)
	if err != nil {
		log.Error("Error writing response:", err)
	}
}

// Gen handles the HTTP POST request to create the files with their ids
func Gen(w http.ResponseWriter, r *http.Request, cfg config.MutualPeersConfig) {
	var body RequestBody
	var resp Response

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		log.Error("Error decoding the request body into the struct:", err)
		resp := Response{
			Status: http.StatusInternalServerError,
			Body:   body.Body,
			Errors: err,
		}
		ReturnResponse(resp, w, r)
	}

	// verify that the node is in the config
	ok, peer := nodes.ValidateNode(body.Body, cfg)
	if !ok {
		log.Error("Error: Pod doesn't exists in the config")
		resp := Response{
			Status: http.StatusNotFound,
			Body:   body.Body,
			Errors: errors.New("error: Pod doesn't exists in the config"),
		}
		ReturnResponse(resp, w, r)
	}

	log.Info("Pod to setup: ", "[", peer.NodeName, "]")

	resp = ConfigureNode(cfg, peer, err, resp)

	ReturnResponse(resp, w, r)
}

func ConfigureNode(
	cfg config.MutualPeersConfig,
	peer config.Peer,
	err error,
	resp Response,
) Response {
	// Get the default values in case we need
	switch peer.NodeType {
	case "da":
		peer = nodes.SetDaNodeDefault(peer)
	case "consensus":
		peer = nodes.SetConsNodeDefault(peer)
	}

	// check if the node uses env var
	if peer.ConnectsAsEnvVar {
		log.Info("Pod: [", peer.NodeName, "] ", "uses env var to connect.")
		// configure the env vars for the node
		err = nodes.SetupNodesEnvVarAndConnections(peer, cfg)
		if err != nil {
			log.Error("Error: ", err)
			resp := Response{
				Status: http.StatusInternalServerError,
				Body:   peer.NodeName,
				Errors: err,
			}
			return resp
		}
	}

	// Configure DA Nodes with which are not using env var
	if peer.NodeType == "da" && !peer.ConnectsAsEnvVar {
		err := nodes.SetupDANodeWithConnections(peer, cfg)
		if err != nil {
			log.Error("Error: ", err)
		}
	}

	//if len(peer.ConnectsTo) > 0 {
	//	log.Info("Pod: [", peer.NodeName, "] ", "uses list of connections.")
	//	// if the node doesn't use env vars, let's use the multi address
	//	output, err := nodes.GenerateTrustedPeersAddr(cfg, peer)
	//	if err != nil {
	//		log.Error("Error: ", err)
	//		resp := Response{
	//			Status: http.StatusInternalServerError,
	//			Body:   peer.NodeName,
	//			Errors: err,
	//		}
	//		return resp
	//	}
	//	// print the output -> should be the nodeId
	//	log.Info(output)
	//
	//	nodeId := make(map[string]string)
	//	nodeId[peer.NodeName] = output
	//
	//	resp = Response{
	//		Status: http.StatusOK,
	//		Body:   nodeId,
	//		Errors: nil,
	//	}
	//}

	return resp
}

// GenAll generate the list of ids for all the nodes availabe in the config
func GenAll(w http.ResponseWriter, r *http.Request, cfg config.MutualPeersConfig) {
	var body RequestMultipleNodesBody
	var resp Response

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		log.Error("Error decoding the request body into the struct:", err)
		resp := Response{
			Status: http.StatusInternalServerError,
			Body:   body.Body,
			Errors: err,
		}
		ReturnResponse(resp, w, r)
	}

	pod := body.Body
	log.Info(pod)

	nodeIDs, err := nodes.GenerateAllTrustedPeersAddr(cfg, pod)
	if err != nil {
		log.Error("Error: ", err)
		// resp -> generate the response with the error
		resp := Response{
			Status: http.StatusInternalServerError,
			Body:   pod,
			Errors: err,
		}
		ReturnResponse(resp, w, r)
	}

	// remove if the ids is empty
	for nodeName, id := range nodeIDs {
		log.Info("node from redis:", nodeName, " ", id)
		if id == "" {
			// if the id is empty, we remove it from the map
			delete(nodeIDs, nodeName)
		}
	}

	// resp -> generate the response
	resp = Response{
		Status: http.StatusOK,
		Body:   nodeIDs,
		Errors: nil,
	}
	ReturnResponse(resp, w, r)
}

// ReturnResponse assert function to write the reponse
func ReturnResponse(resp Response, w http.ResponseWriter, r *http.Request) {
	jsonData, err := json.Marshal(resp)
	if err != nil {
		log.Error("Error marshaling to JSON:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// write all the headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonData)
	if err != nil {
		log.Error("Error writing response:", err)
	}
}

// logRequest is a middleware function that logs the incoming request.
func LogRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info(r.Method, " ", r.URL.Path)
		handler.ServeHTTP(w, r)
	})
}
