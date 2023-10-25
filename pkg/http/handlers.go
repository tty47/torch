package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/celestiaorg/torch/config"
	"github.com/celestiaorg/torch/pkg/db/redis"
	"github.com/celestiaorg/torch/pkg/nodes"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// errorMsg common error message.
const errorMsg = "Error: "

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

// GetConfig handles the HTTP GET request for retrieving the config as JSON.
func GetConfig(w http.ResponseWriter, cfg config.MutualPeersConfig) {
	// Generate the response, including the configuration
	resp := Response{
		Status: http.StatusOK,
		Body:   cfg,
		Errors: nil,
	}

	ReturnResponse(resp, w)
}

// List handles the HTTP GET request for retrieving the list of matching pods as JSON.
func List(w http.ResponseWriter) {
	red := redis.InitRedisConfig()
	ctx := context.TODO()

	// get all values from redis
	nodeIDs, err := red.GetAllKeys(ctx)
	if err != nil {
		log.Error("Error getting the keys and values: ", err)
	}

	// Generate the response, including the configuration
	resp := Response{
		Status: http.StatusOK,
		Body:   nodeIDs,
		Errors: nil,
	}

	ReturnResponse(resp, w)
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
		log.Error(errorMsg, "Pod doesn't exists in the config")
		resp := Response{
			Status: http.StatusNotFound,
			Body:   peer.NodeName,
			Errors: errors.New("error: Pod doesn't exists in the config"),
		}
		ReturnResponse(resp, w)
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

	// Generate the response, including the configuration
	resp = Response{
		Status: http.StatusOK,
		Body:   nodeIDs,
		Errors: nil,
	}

	ReturnResponse(resp, w)
}

// Gen handles the HTTP POST request to create the files with their ids.
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
		ReturnResponse(resp, w)
	}

	// verify that the node is in the config
	ok, peer := nodes.ValidateNode(body.Body, cfg)
	if !ok {
		log.Error(errorMsg, "Pod doesn't exists in the config")
		resp := Response{
			Status: http.StatusNotFound,
			Body:   body.Body,
			Errors: errors.New("error: Pod doesn't exists in the config"),
		}
		ReturnResponse(resp, w)
	}

	log.Info("Pod to setup: ", "[", peer.NodeName, "]")

	resp = ConfigureNode(cfg, peer, err)

	ReturnResponse(resp, w)
}

func ConfigureNode(
	cfg config.MutualPeersConfig,
	peer config.Peer,
	err error,
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
			log.Error(errorMsg, err)
			return Response{
				Status: http.StatusInternalServerError,
				Body:   peer.NodeName,
				Errors: err,
			}
		}
	}

	// Configure DA Nodes with which are not using env var
	if peer.NodeType == "da" && !peer.ConnectsAsEnvVar {
		err := nodes.SetupDANodeWithConnections(peer)
		if err != nil {
			log.Error(errorMsg, err)
			return Response{
				Status: http.StatusInternalServerError,
				Body:   peer.NodeName,
				Errors: err,
			}
		}
	}

	// return the resp with status 200 and the node name.
	return Response{
		Status: http.StatusOK,
		Body:   peer.NodeName,
		Errors: "",
	}
}

// comment this endpoint for now.
//// GenAll generate the list of ids for all the nodes available in the config.
//func GenAll(w http.ResponseWriter, r *http.Request, cfg config.MutualPeersConfig) {
//	var body RequestMultipleNodesBody
//	var resp Response
//
//	err := json.NewDecoder(r.Body).Decode(&body)
//	if err != nil {
//		log.Error("Error decoding the request body into the struct:", err)
//		resp := Response{
//			Status: http.StatusInternalServerError,
//			Body:   body.Body,
//			Errors: err,
//		}
//		ReturnResponse(resp, w)
//	}
//
//	pod := body.Body
//	log.Info(pod)
//
//	nodeIDs, err := nodes.GenerateAllTrustedPeersAddr(cfg, pod)
//	if err != nil {
//		log.Error(errorMsg, err)
//		// resp -> generate the response with the error
//		resp := Response{
//			Status: http.StatusInternalServerError,
//			Body:   pod,
//			Errors: err,
//		}
//		ReturnResponse(resp, w)
//	}
//
//	// remove if the ids is empty
//	for nodeName, id := range nodeIDs {
//		log.Info("node from redis:", nodeName, " ", id)
//		if id == "" {
//			// if the id is empty, we remove it from the map
//			delete(nodeIDs, nodeName)
//		}
//	}
//
//	// resp -> generate the response
//	resp = Response{
//		Status: http.StatusOK,
//		Body:   nodeIDs,
//		Errors: nil,
//	}
//	ReturnResponse(resp, w)
//}

// ReturnResponse assert function to write the response.
func ReturnResponse(resp Response, w http.ResponseWriter) {
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

// LogRequest is a middleware function that logs the incoming request.
func LogRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info(r.Method, " ", r.URL.Path)
		handler.ServeHTTP(w, r)
	})
}
