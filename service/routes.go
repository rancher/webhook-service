package service

import (
	"crypto/rsa"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/api"
	v1client "github.com/rancher/go-rancher/client"
	"github.com/rancher/webhook-service/drivers"
	"github.com/rancher/webhook-service/model"
)

var schemas *v1client.Schemas

func HandleError(s *v1client.Schemas, t func(http.ResponseWriter, *http.Request) (int, error)) http.Handler {
	return api.ApiHandler(s, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if code, err := t(rw, req); err != nil {
			apiContext := api.GetApiContext(req)
			logrus.Errorf("Error in request: %v", err)
			rw.WriteHeader(code)
			writeErr := apiContext.WriteResource(&model.ServerAPIError{
				Resource: v1client.Resource{
					Type: "error",
				},
				Code:    code,
				Status:  "Server Error",
				Message: err.Error(),
			})
			if writeErr != nil {
				logrus.Errorf("Failed to write err: %v", err)
			}
		}
	}))
}

type RouteHandler struct {
	ClientFactory RancherClientFactory
	PrivateKey    *rsa.PrivateKey
	PublicKey     *rsa.PublicKey
}

func NewRouter(r *RouteHandler) *mux.Router {
	schemas = driverSchemas()
	router := mux.NewRouter().StrictSlash(true)
	f := HandleError
	router.Methods("POST").Path("/v1-webhooks").Handler(f(schemas, r.ConstructPayload))
	router.Methods("GET").Path("/v1-webhooks").Handler(f(schemas, r.ListWebhooks))
	router.Methods("GET").Path("/v1-webhooks/{id}").Handler(f(schemas, r.GetWebhook))
	router.Methods("DELETE").Path("/v1-webhooks/{id}").Handler(f(schemas, r.DeleteWebhook))
	router.Methods("POST").Path("/v1-webhooks-receiver").Handler(f(schemas, r.Execute))
	router.Methods("GET").Path("/v1-webhooks/schemas").Handler(api.SchemasHandler(schemas))

	return router
}

func driverSchemas() *v1client.Schemas {
	schemas := &v1client.Schemas{}

	for key, value := range drivers.Drivers {
		schemas.AddType(key, value.GetSchema())
	}

	schemas.AddType("apiVersion", v1client.Resource{})
	schemas.AddType("schema", v1client.Schema{})
	schemas.AddType("error", model.ServerAPIError{})
	schemas.AddType("webhook", model.Webhook{})
	return schemas
}
