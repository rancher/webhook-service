package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
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

	projectID, errCode, err := getProjectID(r)
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

	code, err := rh.isUniqueName(wh.Name, projectID, apiClient)
	if err != nil {
		return code, err
	}

	code, err = driver.ValidatePayload(driverConfig, apiClient)
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

	url := apiContext.UrlBuilder.Version("v1-webhooks")
	url = url + "/endpoint?token="
	jwt = url + jwt

	//saveWebhook needs only user fields
	webhook, err := saveWebhook(uuid, wh.Name, wh.Driver, jwt, driverConfig, apiClient)
	if err != nil {
		return 500, err
	}

	//needs only user fields
	whResponse, err := newWebhook(apiContext, jwt, webhook.Id, wh.Driver, wh.Name, driverConfig, driver,
		webhook.State, r)
	if err != nil {
		return 500, errors.Wrap(err, "Unable to create webhook response")
	}
	apiContext.WriteResource(whResponse)
	return 200, nil
}

func saveWebhook(uuid string, name string, driver string, url string, config interface{}, apiClient client.RancherClient) (*client.GenericObject, error) {
	resourceData := map[string]interface{}{
		"url":    url,
		"driver": driver,
		"config": config,
	}
	obj, err := apiClient.GenericObject.Create(&client.GenericObject{
		Name:         name,
		Key:          uuid,
		ResourceData: resourceData,
		Kind:         "webhookReceiver",
	})

	if err != nil {
		return &client.GenericObject{}, fmt.Errorf("Failed to create webhook : %v", err)
	}
	return obj, nil
}

func getDriverConfig(wh *model.Webhook) interface{} {
	r := reflect.ValueOf(wh)
	fieldName := strings.Title(wh.Driver) + "Config"
	f := reflect.Indirect(r).FieldByName(fieldName)
	return f.Interface()
}
