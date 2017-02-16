package service

import (
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	v1client "github.com/rancher/go-rancher/client"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/webhook-service/drivers"
	"github.com/rancher/webhook-service/model"
)

func (rh *RouteHandler) ListWebhooks(w http.ResponseWriter, r *http.Request) (int, error) {
	logrus.Infof("Listing webhooks")
	apiContext := api.GetApiContext(r)
	projectID, errCode, err := getProjectID(r)
	if err != nil {
		return errCode, err
	}
	apiClient, err := rh.ClientFactory.GetClient(projectID)
	if err != nil {
		return 500, err
	}
	filters := make(map[string]interface{})
	filters["kind"] = "webhookReceiver"
	objs, err := apiClient.GenericObject.List(&client.ListOpts{
		Filters: filters,
	})
	response := []model.Webhook{}
	for _, obj := range objs.Data {
		webhook, err := rh.convertToWebhookGenericObject(obj)
		if err != nil {
			logrus.Warnf("Skipping webhook %s because: %v", obj.Id, err)
			continue
		}

		driver := drivers.GetDriver(webhook.Driver)
		if driver == nil {
			logrus.Warnf("Skipping webhook %s because driver cannot be located", webhook.ID)
			continue
		}
		respWebhook, err := newWebhook(apiContext, webhook.URL, webhook.ID, webhook.Driver, webhook.Name,
			webhook.Config, driver, webhook.State, r)
		if err != nil {
			logrus.Warnf("Skipping webhook %s an error ocurred while producing response: %v", webhook.ID, err)
			continue
		}

		response = append(response, *respWebhook)
	}

	collectionURL := apiContext.UrlBuilder.Current() + "?projectId=" + projectID
	apiContext.Write(&model.WebhookCollection{
		Collection: v1client.Collection{
			ResourceType: "receiver",
			Links:        map[string]string{"self": collectionURL}},
		Data: response})
	return 200, nil
}

func (rh *RouteHandler) GetWebhook(w http.ResponseWriter, r *http.Request) (int, error) {
	apiContext := api.GetApiContext(r)
	vars := mux.Vars(r)
	webhookID := vars["id"]
	logrus.Infof("Getting webhook %v", webhookID)

	projectID, errCode, err := getProjectID(r)
	if err != nil {
		return errCode, err
	}
	apiClient, err := rh.ClientFactory.GetClient(projectID)
	if err != nil {
		return 500, err
	}
	obj, err := apiClient.GenericObject.ById(webhookID)
	if err != nil {
		return 500, err
	}

	if obj == nil {
		return 404, fmt.Errorf("Webhook not found")
	}

	webhook, err := rh.convertToWebhookGenericObject(*obj)
	if err != nil {
		return 500, err
	}

	driver := drivers.GetDriver(webhook.Driver)
	if driver == nil {
		return 400, fmt.Errorf("Can't find driver %v", webhook.Driver)
	}

	respWebhook, err := newWebhook(apiContext, webhook.URL, webhook.ID, webhook.Driver, webhook.Name,
		webhook.Config, driver, webhook.State, r)
	if err != nil {
		return 500, errors.Wrap(err, "Unable to create webhook response")
	}

	apiContext.WriteResource(respWebhook)
	return 200, nil
}

func (rh *RouteHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) (int, error) {
	vars := mux.Vars(r)
	webhookID := vars["id"]

	projectID, errCode, err := getProjectID(r)
	if err != nil {
		return errCode, err
	}

	apiClient, err := rh.ClientFactory.GetClient(projectID)
	if err != nil {
		return 500, err
	}
	obj, err := apiClient.GenericObject.ById(webhookID)
	if err != nil {
		return 500, err
	}

	if obj == nil {
		return 404, fmt.Errorf("Webhook not found")
	}

	err = apiClient.GenericObject.Delete(obj)
	if err != nil {
		statusCode := err.(*client.ApiError).StatusCode
		return statusCode, err
	}
	return 204, nil
}

func getProjectID(r *http.Request) (string, int, error) {
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		return "", 400, fmt.Errorf("projectId must be supplied as query parameter")
	}

	return projectID, 0, nil
}

func newWebhook(context *api.ApiContext, url string, id string, driverName string, name string,
	driverConfig interface{}, driver drivers.WebhookDriver, state string, r *http.Request) (*model.Webhook, error) {

	selfLink := context.UrlBuilder.ReferenceByIdLink("receiver", id)
	projectID := r.URL.Query().Get("projectId")
	if projectID != "" {
		selfLink = selfLink + "?projectId=" + projectID
	}

	webhook := &model.Webhook{
		Resource: v1client.Resource{
			Id:    id,
			Type:  "receiver",
			Links: map[string]string{"self": selfLink},
		},
		URL:    url,
		Driver: driverName,
		Name:   name,
		State:  state,
	}
	driver.ConvertToConfigAndSetOnWebhook(driverConfig, webhook)
	return webhook, nil
}

type webhookGenericObject struct {
	ID     string
	Name   string
	State  string
	Links  map[string]string
	Driver string
	URL    string
	Key    string
	Config interface{}
}

func (rh *RouteHandler) convertToWebhookGenericObject(genericObject client.GenericObject) (webhookGenericObject, error) {
	d, ok := genericObject.ResourceData["driver"].(string)
	if !ok {
		return webhookGenericObject{}, fmt.Errorf("Couldn't read webhook data. Bad driver")
	}

	url, ok := genericObject.ResourceData["url"].(string)
	if !ok {
		return webhookGenericObject{}, fmt.Errorf("Couldn't read webhook data. Bad url")
	}

	config, ok := genericObject.ResourceData["config"]
	if !ok {
		return webhookGenericObject{}, fmt.Errorf("Couldn't read webhook data. Bad config on resource")
	}

	return webhookGenericObject{
		Name:   genericObject.Name,
		ID:     genericObject.Id,
		State:  genericObject.State,
		Links:  genericObject.Links,
		Driver: d,
		URL:    url,
		Key:    genericObject.Key,
		Config: config,
	}, nil
}

func (rh *RouteHandler) isUniqueName(webhookName string, projectID string, apiClient *client.RancherClient) (int, error) {
	filters := make(map[string]interface{})
	filters["name"] = webhookName
	obj, err := apiClient.GenericObject.List(&client.ListOpts{
		Filters: filters,
	})
	if err != nil {
		return 500, err
	}
	if len(obj.Data) > 0 {
		return 400, fmt.Errorf("Cannot have duplicate webhook name, webhook %s already exists", webhookName)
	}
	return 200, nil
}
