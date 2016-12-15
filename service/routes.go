package service

import (
	"crypto/rsa"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/webhook-service/drivers"
	//This should be v2, supporting schemas
	"github.com/rancher/go-rancher/client"
	"github.com/rancher/webhook-service/model"
)

var schemas *client.Schemas

func HandleError(s *client.Schemas, t func(http.ResponseWriter, *http.Request) (int, error)) http.Handler {
	return api.ApiHandler(s, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if code, err := t(rw, req); err != nil {
			apiContext := api.GetApiContext(req)
			logrus.Errorf("Error in request: %v", err)
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

func driverSchemas() *client.Schemas {
	schemas := &client.Schemas{}

	for key, value := range drivers.Drivers {
		schemas.AddType(key, value.GetSchema())
	}

	schemas.AddType("apiVersion", client.Resource{})
	schemas.AddType("schema", client.Schema{})
	schemas.AddType("error", model.ServerAPIError{})
	schemas.AddType("webhook", model.Webhook{})
	return schemas
}

func newWebhook(context *api.ApiContext, url string, links map[string]string, id string, driver string, name string, userConfig interface{}) *model.Webhook {
	webhook := &model.Webhook{
		Resource: client.Resource{
			Id:    id,
			Type:  "webhook",
			Links: links,
		},
		URL:    url,
		Driver: driver,
		Name:   name,
	}
	ConfigName := driver + "Config"
	switch ConfigName {
	case "scaleServiceConfig":
		webhook.ScaleServiceConfig = userConfig.(model.ScaleService)
	}

	return webhook
}
