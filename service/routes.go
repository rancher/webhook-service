package service

import (
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	//This should be v2, supporting schemas
	"github.com/rancher/webhook-service/drivers"
	"net/http"
)

var schemas *client.Schemas
var router *mux.Router

func HandleError(s *client.Schemas, t func(http.ResponseWriter, *http.Request) (int, error)) http.Handler {
	return api.ApiHandler(s, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if code, err := t(rw, req); err != nil {
			apiContext := api.GetApiContext(req)
			logrus.Errorf("Error in request: %v", err)
			writeErr := apiContext.WriteResource(&ServerAPIError{
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
	rcf RancherClientFactory
}

func NewRouter() *mux.Router {
	schemas = driverSchemas()
	router = mux.NewRouter().StrictSlash(true)
	f := HandleError
	r := RouteHandler{}
	r.rcf = &ExecuteStruct{}
	router.Methods("POST").Path("/v1-webhooks-generate").Handler(f(schemas, r.ConstructPayload))
	router.Methods("POST").Path("/v1-webhooks-receiver").Handler(f(schemas, r.Execute))
	router.Methods("GET").Path("/v1-webhooks-generate/schemas").Handler(api.SchemasHandler(schemas))

	return router
}

func driverSchemas() *client.Schemas {
	schemas := &client.Schemas{}

	for key, value := range drivers.Drivers {
		schemas.AddType(key, value.GetSchema())
	}

	schemas.AddType("apiVersion", client.Resource{})
	schemas.AddType("schema", client.Schema{})
	schemas.AddType("error", ServerAPIError{})
	schemas.AddType("generatedWebhook", generatedWebhook{})

	return schemas
}

type ServerAPIError struct {
	client.Resource
	Code    int    `json:"statusCode"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type generatedWebhook struct {
	client.Resource
	URL string `json:"url"`
}

func newGeneratedWebhook(context *api.ApiContext, url string) *generatedWebhook {
	response := &generatedWebhook{
		Resource: client.Resource{
			Id:   "name",
			Type: "generatedWebhook",
		},
		URL: url,
	}
	return response
}

type ExecuteStruct struct{}
