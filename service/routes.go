package service

import (
	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	//This should be v2, supporting schemas
	"net/http"
)

var schemas *client.Schemas
var router *mux.Router

func NewRouter() *mux.Router {
	schemas = &client.Schemas{}
	router = mux.NewRouter().StrictSlash(true)
	router.Methods("POST").Path("/v1-webhooks-generate").Handler(api.ApiHandler(schemas, http.HandlerFunc(ConstructPayload)))
	router.Methods("POST").Path("/v1-webhooks-receiver").Handler(api.ApiHandler(schemas, http.HandlerFunc(Execute)))
	router.Methods("GET").Path("/v1-webhooks-generate/schemas").Handler(api.ApiHandler(schemas, http.HandlerFunc(DisplaySchemas)))
	return router
}
