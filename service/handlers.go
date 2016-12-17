package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"

	"github.com/Sirupsen/logrus"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	v1client "github.com/rancher/go-rancher/client"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/rancher-auth-service/util"
	"github.com/rancher/webhook-service/drivers"
	"github.com/rancher/webhook-service/model"
)

func (rh *RouteHandler) ConstructPayload(w http.ResponseWriter, r *http.Request) (int, error) {
	apiContext := api.GetApiContext(r)
	wh := &model.Webhook{}
	logrus.Infof("Construct Payload")
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return 500, err
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return 400, fmt.Errorf("Content-Type must be supplied as header. Only application/json is supported")
	}

	projectID, errCode, err := getProjectIDFromHeader(r)
	if err != nil {
		return errCode, err
	}

	if err := json.Unmarshal(bytes, &wh); err != nil {
		return 400, errors.Wrap(err, "Bad request body")
	}

	if wh.Name == "" {
		return 400, fmt.Errorf("Name not provided")
	}

	if wh.Driver == "" {
		return 400, fmt.Errorf("Driver not provided")
	}

	driverConfig := getDriverConfig(wh)
	if driverConfig == nil {
		return 400, fmt.Errorf("Invalid driver %v", wh.Driver)
	}

	driver := drivers.GetDriver(wh.Driver)
	if driver == nil {
		return 400, fmt.Errorf("Invalid driver %v", wh.Driver)
	}

	apiClient, err := rh.ClientFactory.GetClient(projectID)
	if err != nil {
		return 500, err
	}

	code, err := driver.ValidatePayload(driverConfig, apiClient)
	if err != nil {
		return code, err
	}

	uuid := uuid.New()
	config := map[string]interface{}{
		"projectId": projectID,
		"uuid":      uuid,
		"driver":    wh.Driver,
		"config":    driverConfig,
	}
	jwt, err := util.CreateTokenWithPayload(config, rh.PrivateKey)
	if err != nil {
		return 500, err
	}

	url := baseURL(r)
	url = url + "/v1-webhooks-receiver?token="
	jwt = url + jwt

	//saveWebhook needs only user fields
	webhook, err := saveWebhook(uuid, wh.Name, wh.Driver, jwt, driverConfig, apiClient)
	if err != nil {
		return 500, err
	}

	//needs only user fields
	webhook.Links["self"] = baseURL(r) + r.URL.String() + "/" + webhook.Id
	whResponse, err := newWebhook(apiContext, jwt, webhook.Links, webhook.Id, wh.Driver, wh.Name, driverConfig, driver,
		webhook.State)
	if err != nil {
		return 500, errors.Wrap(err, "Unable to create webhook response")
	}
	apiContext.WriteResource(whResponse)
	return 200, nil
}

func baseURL(r *http.Request) string {
	var url string
	protocol := r.Header.Get("X-Forwarded-Proto")
	if protocol != "" {
		url = protocol + "://"
	} else {
		url = "http://"
	}
	url = url + r.Host
	return url
}

func (rh *RouteHandler) Execute(w http.ResponseWriter, r *http.Request) (int, error) {
	jwtSigned := r.FormValue("token")
	token, err := jwt.Parse(jwtSigned, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return rh.PublicKey, nil
	})

	if err != nil || !token.Valid {
		return 500, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		driverID, ok := claims["driver"].(string)
		if !ok {
			return 400, fmt.Errorf("Driver not found after decode")
		}

		driver := drivers.GetDriver(driverID)
		if driver == nil {
			return 400, fmt.Errorf("Driver %s is not registered", driverID)
		}

		projectID, ok := claims["projectId"].(string)
		if !ok {
			return 400, fmt.Errorf("Project not provided by server")
		}

		uuid, ok := claims["uuid"].(string)
		if !ok {
			return 400, fmt.Errorf("Uuid not found after decode")
		}

		apiClient, err := rh.ClientFactory.GetClient(projectID)
		if err != nil {
			return 500, err
		}

		code, err := validateWebhook(uuid, apiClient)
		if err != nil {
			return code, err
		}

		responseCode, err := driver.Execute(claims["config"], apiClient)
		if err != nil {
			return responseCode, fmt.Errorf("Error %v in executing driver for %s", err, driverID)
		}
	}
	return 200, nil
}

func (rh *RouteHandler) ListWebhooks(w http.ResponseWriter, r *http.Request) (int, error) {
	apiContext := api.GetApiContext(r)
	projectID, errCode, err := getProjectIDFromHeader(r)
	if err != nil {
		return errCode, err
	}
	apiClient, err := rh.ClientFactory.GetClient(projectID)
	if err != nil {
		return 500, err
	}
	webhooks, err := apiClient.Webhook.List(&client.ListOpts{})
	response := []model.Webhook{}
	for _, webhook := range webhooks.Data {
		driver := drivers.GetDriver(webhook.Driver)
		if driver == nil {
			logrus.Warnf("Skipping webhook %#v because driver cannot be located", webhook)
			continue
		}
		webhook.Links["self"] = baseURL(r) + r.URL.String() + "/" + webhook.Id
		respWebhook, err := newWebhook(apiContext, webhook.Url, webhook.Links, webhook.Id, webhook.Driver, webhook.Name,
			webhook.Config, driver, webhook.State)
		if err != nil {
			logrus.Warnf("Skipping webhook %#v an error ocurred while producing response: %v", webhook, err)
			continue
		}

		response = append(response, *respWebhook)
	}
	apiContext.Write(&model.WebhookCollection{Data: response})
	return 200, nil
}

func (rh *RouteHandler) GetWebhook(w http.ResponseWriter, r *http.Request) (int, error) {
	apiContext := api.GetApiContext(r)
	vars := mux.Vars(r)
	webhookID := vars["id"]
	logrus.Infof("Getting webhook %v", webhookID)

	projectID, errCode, err := getProjectIDFromHeader(r)
	if err != nil {
		return errCode, err
	}
	apiClient, err := rh.ClientFactory.GetClient(projectID)
	if err != nil {
		return 500, err
	}
	webhook, err := apiClient.Webhook.ById(webhookID)
	if err != nil {
		return 500, err
	}

	if webhook == nil {
		return 404, fmt.Errorf("Webhook not found")
	}

	driver := drivers.GetDriver(webhook.Driver)
	if driver == nil {
		return 500, fmt.Errorf("Can't find driver %v", webhook.Driver)
	}

	webhook.Links["self"] = baseURL(r) + r.URL.String()
	respWebhook, err := newWebhook(apiContext, webhook.Url, webhook.Links, webhook.Id, webhook.Driver, webhook.Name,
		webhook.Config, driver, webhook.State)
	if err != nil {
		return 500, errors.Wrap(err, "Unable to create webhook response")
	}

	apiContext.WriteResource(respWebhook)
	return 200, nil
}

func (rh *RouteHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) (int, error) {
	vars := mux.Vars(r)
	webhookID := vars["id"]

	projectID, errCode, err := getProjectIDFromHeader(r)
	if err != nil {
		return errCode, err
	}

	apiClient, err := rh.ClientFactory.GetClient(projectID)
	if err != nil {
		return 500, err
	}
	webhook, err := apiClient.Webhook.ById(webhookID)
	if err != nil {
		return 500, err
	}

	if webhook == nil {
		return 404, fmt.Errorf("Webhook not found")
	}

	err = apiClient.Webhook.Delete(webhook)
	if err != nil {
		return 500, err
	}
	return 200, nil
}

func getProjectIDFromHeader(r *http.Request) (string, int, error) {
	projectID := r.Header.Get("X-API-Project-Id")
	if projectID == "" {
		return "", 400, fmt.Errorf("Project id must be supplied in X-API-Project-Id request header")
	}

	return projectID, 0, nil
}

func saveWebhook(uuid string, name string, driver string, url string, input interface{}, apiClient client.RancherClient) (*client.Webhook, error) {
	webhook, err := apiClient.Webhook.Create(&client.Webhook{
		Name:   name,
		Key:    uuid,
		Url:    url,
		Config: input,
		Driver: driver,
	})

	if err != nil {
		return &client.Webhook{}, fmt.Errorf("Failed to create webhook : %v", err)
	}
	return webhook, nil
}

func validateWebhook(uuid string, apiClient client.RancherClient) (int, error) {
	filters := make(map[string]interface{})
	filters["key"] = uuid
	webhookCollection, err := apiClient.Webhook.List(&client.ListOpts{
		Filters: filters,
	})
	if err != nil {
		return 500, err
	}
	if len(webhookCollection.Data) > 0 {
		return 0, nil
	}
	return 403, fmt.Errorf("Requested webhook has been revoked")
}

func getDriverConfig(wh *model.Webhook) interface{} {
	r := reflect.ValueOf(wh)
	f := reflect.Indirect(r).FieldByName(getDriverConfigFieldName(wh.Driver))
	return f.Interface()
}

func getDriverConfigFieldName(driver string) string {
	return strings.Title(driver) + "Config"
}

func newWebhook(context *api.ApiContext, url string, links map[string]string, id string, driverName string, name string,
	driverConfig interface{}, driver drivers.WebhookDriver, state string) (*model.Webhook, error) {
	webhook := &model.Webhook{
		Resource: v1client.Resource{
			Id:    id,
			Type:  "webhook",
			Links: links,
		},
		URL:    url,
		Driver: driverName,
		Name:   name,
		State:  state,
	}
	driver.ConvertToConfigAndSetOnWebhook(driverConfig, webhook)
	return webhook, nil
}
