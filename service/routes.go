package service

import (
	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	"net/http"
)

var schemas *client.Schemas
var router *mux.Router

func NewRouter() *mux.Router {
	schemas = &client.Schemas{}
	router = mux.NewRouter().StrictSlash(true)
	router.Methods("POST").Path("/v2-beta/webhook/construct").Handler(api.ApiHandler(schemas, http.HandlerFunc(Construct)))
	router.Methods("GET").Path("/v2-beta/webhook/construct").Handler(api.ApiHandler(schemas, http.HandlerFunc(Construct)))
	router.Methods("POST").Path("/v2-beta/webhook/execute").Handler(api.ApiHandler(schemas, http.HandlerFunc(Execute)))
	router.Methods("GET").Path("/v2-beta/webhook/execute").Handler(api.ApiHandler(schemas, http.HandlerFunc(Execute)))

	return router
}
