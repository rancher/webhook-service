package service

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/mitchellh/mapstructure"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/rancher-auth-service/util"
	"github.com/rancher/webhook-service/config"
	"github.com/rancher/webhook-service/drivers"
	"github.com/urfave/cli"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type RancherClientFactory interface {
	GetClient(projectID string) (client.RancherClient, error)
}

func (rh *RouteHandler) ConstructPayload(w http.ResponseWriter, r *http.Request) (int, error) {
	apiContext := api.GetApiContext(r)
	wh := &webhook{}
	var url string
	logrus.Infof("Construct Payload")
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return 500, err
	}

	if err := json.Unmarshal(bytes, &wh); err != nil {
		return 400, errors.Wrap(err, "Bad request body")
	}

	projectID := r.Header.Get("X-API-Project-Id")
	if projectID == "" {
		return 500, fmt.Errorf("Project ID not obtained from cattle")
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

	apiClient, err := rh.rcf.GetClient(projectID)
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
	jwt, err := util.CreateTokenWithPayload(config, rh.privateKey)
	if err != nil {
		return 500, err
	}

	protocol := r.Header.Get("X-Forwarded-Proto")
	if protocol != "" {
		url = protocol + "://"
	} else {
		url = "http://"
	}
	url = url + r.Host + "/v1-webhooks-receiver?token="
	jwt = url + jwt

	//saveWebhook needs only user fields
	webhook, err := saveWebhook(uuid, wh.Name, wh.Driver, jwt, driverConfig, apiClient)
	if err != nil {
		return 500, err
	}

	//needs only user fields
	apiContext.WriteResource(newWebhook(apiContext, jwt, webhook.Links, webhook.Id, wh.Driver,
		wh.Name, driverConfig))
	return 200, nil
}

func (rh *RouteHandler) Execute(w http.ResponseWriter, r *http.Request) (int, error) {
	jwtSigned := r.FormValue("token")
	token, err := jwt.Parse(jwtSigned, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return rh.publicKey, nil
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

		apiClient, err := rh.rcf.GetClient(projectID)
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
	projectID := r.Header.Get("X-API-Project-Id")
	if projectID == "" {
		return 400, fmt.Errorf("Project ID not obtained from cattle")
	}
	apiClient, err := rh.rcf.GetClient(projectID)
	if err != nil {
		return 500, err
	}
	webhooks, err := apiClient.Webhook.List(&client.ListOpts{})
	response := []webhook{}
	for _, webhook := range webhooks.Data {
		config := getConfig(webhook.Driver)
		err = mapstructure.Decode(webhook.Config, &config)
		if err != nil {
			return 500, err
		}
		respWebhook := newWebhook(apiContext, webhook.Url, webhook.Links, webhook.Id, webhook.Driver, webhook.Name, config)
		response = append(response, *respWebhook)
	}
	apiContext.Write(&webhookCollection{Data: response})
	return 200, nil
}

func (rh *RouteHandler) GetWebhook(w http.ResponseWriter, r *http.Request) (int, error) {
	apiContext := api.GetApiContext(r)
	vars := mux.Vars(r)
	webhookID := vars["id"]
	projectID := r.Header.Get("X-API-Project-Id")
	if projectID == "" {
		return 400, fmt.Errorf("Project ID not obtained from cattle")
	}
	apiClient, err := rh.rcf.GetClient(projectID)
	if err != nil {
		return 500, err
	}
	webhook, err := apiClient.Webhook.ById(webhookID)
	if err != nil {
		return 500, err
	}

	config := getConfig(webhook.Driver)
	err = mapstructure.Decode(webhook.Config, &config)
	if err != nil {
		return 500, err
	}
	respWebhook := newWebhook(apiContext, webhook.Url, webhook.Links, webhook.Id, webhook.Driver, webhook.Name, config)
	apiContext.WriteResource(respWebhook)
	return 200, nil
}

func (rh *RouteHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) (int, error) {
	vars := mux.Vars(r)
	webhookID := vars["id"]
	projectID := r.Header.Get("X-API-Project-Id")
	if projectID == "" {
		return 400, fmt.Errorf("Project ID not obtained from cattle")
	}
	apiClient, err := rh.rcf.GetClient(projectID)
	if err != nil {
		return 500, err
	}
	webhook, err := apiClient.Webhook.ById(webhookID)
	if err != nil {
		return 500, err
	}

	err = apiClient.Webhook.Delete(webhook)
	if err != nil {
		return 500, err
	}
	return 200, nil
}

func (e *ExecuteStruct) GetClient(projectID string) (client.RancherClient, error) {
	config := config.GetConfig()
	url := fmt.Sprintf("%s/projects/%s/schemas", config.CattleURL, projectID)
	apiClient, err := client.NewRancherClient(&client.ClientOpts{
		Timeout:   time.Second * 30,
		Url:       url,
		AccessKey: config.CattleAccessKey,
		SecretKey: config.CattleSecretKey,
	})
	if err != nil {
		return client.RancherClient{}, fmt.Errorf("Error in creating API client")
	}
	return *apiClient, nil
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

func getDriverConfig(wh *webhook) interface{} {
	r := reflect.ValueOf(wh)
	f := reflect.Indirect(r).FieldByName(getDriverConfigFieldName(wh.Driver))
	return f.Interface()
}

func getDriverConfigFieldName(driver string) string {
	return strings.Title(driver) + "Config"
}

func getConfig(driver string) interface{} {
	switch getDriverConfigFieldName(driver) {
	case "ScaleServiceConfig":
		return drivers.ScaleService{}
	}
	return nil
}

func GetKeys(c *cli.Context) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	var PrivateKey *rsa.PrivateKey
	var PublicKey *rsa.PublicKey
	privateKeyFile := c.GlobalString("rsa-private-key-file")
	privateKeyFileContents := c.GlobalString("rsa-private-key-contents")

	if privateKeyFile != "" && privateKeyFileContents != "" {
		return nil, nil, fmt.Errorf("Can't specify both, file and contents, halting")
	}
	if privateKeyFile != "" {
		PrivateKey = util.ParsePrivateKey(privateKeyFile)
	} else if privateKeyFileContents != "" {
		PrivateKey = util.ParsePrivateKeyContents(privateKeyFileContents)
	} else {
		return nil, nil, fmt.Errorf("Please provide either rsa-private-key-file or rsa-private-key-contents, halting")
	}

	publicKeyFile := c.GlobalString("rsa-public-key-file")
	publicKeyFileContents := c.GlobalString("rsa-public-key-contents")

	if publicKeyFile != "" && publicKeyFileContents != "" {
		return nil, nil, fmt.Errorf("Can't specify both, file and contents, halting")
	}
	if publicKeyFile != "" {
		PublicKey = util.ParsePublicKey(publicKeyFile)
	} else if publicKeyFileContents != "" {
		PublicKey = util.ParsePublicKeyContents(publicKeyFileContents)
	} else {
		return nil, nil, fmt.Errorf("Please provide either rsa-public-key-file or rsa-public-key-contents, halting")
	}

	return PrivateKey, PublicKey, nil
}
