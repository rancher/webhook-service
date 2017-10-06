package service

import (
	"crypto/rsa"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/v3"
	"github.com/rancher/go-rancher/v3/api"
	"github.com/rancher/webhook-service/drivers"
	"github.com/rancher/webhook-service/model"
)

var schemas *client.Schemas

func HandleError(s *client.Schemas, t func(http.ResponseWriter, *http.Request) (int, error)) http.Handler {
	return api.ApiHandler(s, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if code, err := t(rw, req); err != nil {
			apiContext := api.GetApiContext(req)
			logrus.Errorf("Error in request: %v", err)
			rw.Header().Add("Content-Type", "application/json")
			rw.WriteHeader(code)
			writeErr := apiContext.WriteResource(&model.ServerAPIError{
				Resource: client.Resource{
					Type: "error",
				},
				Code:    code,
				Status:  "Server Error",
				Message: err.Error(),
			})
			if writeErr != nil {
				logrus.Errorf("Failed to write err: %v", err)
			}
		} else {
			if code != 200 {
				rw.WriteHeader(code)
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
	router := mux.NewRouter().StrictSlash(false)
	f := HandleError

	router.Methods("GET").Path("/v1-webhooks").Handler(VersionHandler(schemas))
	router.Methods("GET").Path("/v1-webhooks/").Handler(VersionHandler(schemas))

	router.Methods("GET").Path("/v1-webhooks/schemas/").Handler(api.SchemasHandler(schemas))
	router.Methods("GET").Path("/v1-webhooks/schemas").Handler(api.SchemasHandler(schemas))

	router.Methods("GET").Path("/v1-webhooks/schemas/{id}").Handler(api.SchemaHandler(schemas))
	router.Methods("GET").Path("/v1-webhooks/schemas/{id}/").Handler(api.SchemaHandler(schemas))

	router.Methods("POST").Path("/v1-webhooks/receivers").Handler(f(schemas, r.ConstructPayload))
	router.Methods("POST").Path("/v1-webhooks/receivers/").Handler(f(schemas, r.ConstructPayload))

	router.Methods("GET").Path("/v1-webhooks/receivers").Handler(f(schemas, r.ListWebhooks))
	router.Methods("GET").Path("/v1-webhooks/receivers/").Handler(f(schemas, r.ListWebhooks))

	router.Methods("GET").Path("/v1-webhooks/receivers/{id}").Handler(f(schemas, r.GetWebhook))
	router.Methods("GET").Path("/v1-webhooks/receivers/{id}/").Handler(f(schemas, r.GetWebhook))

	router.Methods("DELETE").Path("/v1-webhooks/receivers/{id}").Handler(f(schemas, r.DeleteWebhook))
	router.Methods("DELETE").Path("/v1-webhooks/receivers/{id}/").Handler(f(schemas, r.DeleteWebhook))

	router.Methods("POST").Path("/v1-webhooks/endpoint").Handler(f(schemas, r.Execute))
	router.Methods("POST").Path("/v1-webhooks/endpoint/").Handler(f(schemas, r.Execute))

	return router
}

func driverSchemas() *client.Schemas {
	schemas := &client.Schemas{}
	webhook := schemas.AddType("receiver", model.Webhook{})
	webhook.CollectionMethods = []string{"GET", "POST"}
	webhook.ResourceMethods = []string{"GET", "DELETE"}

	f := webhook.ResourceFields["name"]
	f.Create = true
	webhook.ResourceFields["name"] = f

	driverOptions := []string{}
	for key, value := range drivers.Drivers {
		webhookField := key + "Config"
		if field, ok := webhook.ResourceFields[webhookField]; ok {
			driverOptions = append(driverOptions, key)
			field.Type = key
			field.Create = true
			webhook.ResourceFields[webhookField] = field
			driverConfig := schemas.AddType(key, value.GetDriverConfigResource())
			driverConfig.CollectionMethods = []string{}
			for k, f := range driverConfig.ResourceFields {
				if k == "hostSelector" {
					f.Create = false
				} else {
					f.Create = true
				}
				driverConfig.ResourceFields[k] = f
			}
			driverConfig = value.CustomizeSchema(driverConfig)
		} else {
			logrus.Warnf("Skipping configured driver %v because it doesn't have a field on webhook", key)
		}
	}

	f = webhook.ResourceFields["driver"]
	f.Create = true
	f.Type = "enum"
	f.Options = driverOptions
	webhook.ResourceFields["driver"] = f

	schemas.AddType("apiVersion", client.Resource{})
	schemas.AddType("schema", client.Schema{})
	schemas.AddType("error", model.ServerAPIError{})

	return schemas
}
