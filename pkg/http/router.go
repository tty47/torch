package handlers

import (
	"net/http"

	"github.com/jrmanes/torch/config"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Router(r *mux.Router, cfg config.MutualPeersConfig) *mux.Router {
	r.Use(LogRequest)
	r.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		GetConfig(w, r, cfg)
	}).Methods("GET")
	r.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		List(w, r, cfg)
	}).Methods("GET")
	r.HandleFunc("/gen", func(w http.ResponseWriter, r *http.Request) {
		Gen(w, r, cfg)
	}).Methods("POST")
	r.HandleFunc("/genAll", func(w http.ResponseWriter, r *http.Request) {
		GenAll(w, r, cfg)
	}).Methods("POST")
	r.Handle("/metrics", promhttp.Handler())

	return r
}
