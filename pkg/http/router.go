package handlers

import (
	"net/http"

	"github.com/celestiaorg/torch/config"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Router(r *mux.Router, cfg config.MutualPeersConfig) *mux.Router {
	r.Use(LogRequest)

	// group the current version to /api/v1
	s := r.PathPrefix("/api/v1").Subrouter()

	// get config
	s.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		GetConfig(w, cfg)
	}).Methods("GET")

	// get nodes
	s.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		List(w)
	}).Methods("GET")
	// get node details by node name
	s.HandleFunc("/noId/{nodeName}", func(w http.ResponseWriter, r *http.Request) {
		GetNoId(w, r, cfg)
	}).Methods("GET")

	// generate
	s.HandleFunc("/gen", func(w http.ResponseWriter, r *http.Request) {
		Gen(w, r, cfg)
	}).Methods("POST")

	// comment this endpoint for now.
	//s.HandleFunc("/genAll", func(w http.ResponseWriter, r *http.Request) {
	//	GenAll(w, r, cfg)
	//}).Methods("POST")

	// metrics
	r.Handle("/metrics", promhttp.Handler())

	return r
}
