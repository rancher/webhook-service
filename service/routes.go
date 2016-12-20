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
	router.Methods("GET").Path("/v1-webhooks/schemas").Handler(api.SchemasHandler(schemas))
	router.Methods("GET").Path("/v1-webhooks/schemas/{id}").Handler(api.SchemaHandler(schemas))
	router.Methods("POST").Path("/v1-webhooks/receivers").Handler(f(schemas, r.ConstructPayload))
	router.Methods("GET").Path("/v1-webhooks/receivers").Handler(f(schemas, r.ListWebhooks))
	router.Methods("GET").Path("/v1-webhooks/receivers/{id}").Handler(f(schemas, r.GetWebhook))
	router.Methods("DELETE").Path("/v1-webhooks/receivers/{id}").Handler(f(schemas, r.DeleteWebhook))
	router.Methods("POST").Path("/v1-webhooks/endpoint").Handler(f(schemas, r.Execute))

	return router
}

func driverSchemas() *v1client.Schemas {
	schemas := &v1client.Schemas{}

	webhook := schemas.AddType("webhookReceiver", model.Webhook{})
	for key, value := range drivers.Drivers {
		webhookField := key + "Config"
		if field, ok := webhook.ResourceFields[webhookField]; ok {
			field.Type = key
			webhook.ResourceFields[webhookField] = field
			driverConfig := schemas.AddType(key, value.GetSchema())
			driverConfig.CollectionMethods = []string{}
		} else {
			logrus.Warnf("Skipping configured driver %v because it doesn't have a field on webhook", key)
		}
	}

	schemas.AddType("apiVersion", v1client.Resource{})
	schemas.AddType("schema", v1client.Schema{})
	schemas.AddType("error", model.ServerAPIError{})

	return schemas
}
