package service

import (
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/pborman/uuid"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/rancher-auth-service/util"
	"github.com/rancher/webhook-service/config"
	"github.com/rancher/webhook-service/drivers"
	"io/ioutil"
	"net/http"
	"time"
)

type RancherClientFactory interface {
	GetClient(projectID string) (client.RancherClient, error)
}

func (rh *RouteHandler) ConstructPayload(w http.ResponseWriter, r *http.Request) (int, error) {
	apiContext := api.GetApiContext(r)
	webhookRequestData := make(map[string]interface{})
	var url string
	logrus.Infof("Construct Payload")
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return 500, err
	}
	json.Unmarshal(bytes, &webhookRequestData)
	accountID := r.Header.Get("X-API-Project-Id")
	if accountID == "" {
		return 500, fmt.Errorf("Project ID not obtained from cattle")
	}
	protocol := r.Header.Get("X-Forwarded-Proto")
	if protocol != "" {
		url = protocol + "://"
	} else {
		url = "http://"
	}
	url = url + r.Host + "/v1-webhooks-receiver?token="
	if _, ok := webhookRequestData["driver"].(string); !ok {
		return 400, fmt.Errorf("Driver of type string not provided")
	}
	driverID := webhookRequestData["driver"].(string)
	driver := drivers.GetDriver(driverID)
	if driver == nil {
		return 400, fmt.Errorf("Driver %s is not registered", driverID)
	}
	apiClient, err := rh.rcf.GetClient(accountID)
	if err != nil {
		return 500, err
	}
	code, err := driver.ValidatePayload(webhookRequestData, apiClient)
	if err != nil {
		return code, err
	}
	webhookRequestData["projectId"] = accountID
	uuid := uuid.New()
	webhookRequestData["uuid"] = uuid
	jwt, err := util.CreateTokenWithPayload(webhookRequestData, PrivateKey)
	if err != nil {
		return 500, err
	}
	name, ok := webhookRequestData["name"].(string)
	if !ok {
		return 400, fmt.Errorf("Name not provided")
	}
	err = saveWebhook(uuid, name, apiClient)
	if err != nil {
		return 500, err
	}
	jwt = url + jwt
	apiContext.WriteResource(newGeneratedWebhook(apiContext, jwt))
	return 200, nil
}

func (rh *RouteHandler) Execute(w http.ResponseWriter, r *http.Request) (int, error) {
	jwtSigned := r.FormValue("token")
	token, err := jwt.Parse(jwtSigned, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return PublicKey, nil
	})

	if err != nil || !token.Valid {
		return 500, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if _, ok := claims["driver"].(string); !ok {
			return 500, fmt.Errorf("Driver not found after decode")
		}
		driverID := claims["driver"].(string)
		driver := drivers.GetDriver(driverID)
		if driver == nil {
			return 400, fmt.Errorf("Driver %s is not registered", driverID)
		}
		if _, ok := claims["projectId"].(string); !ok {
			return 500, fmt.Errorf("AccountId not provided by server")
		}
		projectID := claims["projectId"].(string)
		apiClient, err := rh.rcf.GetClient(projectID)
		if err != nil {
			return 500, err
		}
		uuid, ok := claims["uuid"].(string)
		if !ok {
			return 500, fmt.Errorf("Uuid not found after decode")
		}
		code, err := validateWebhook(uuid, apiClient)
		if err != nil {
			return code, err
		}

		responseCode, err := driver.Execute(claims, apiClient)
		if err != nil {
			return responseCode, fmt.Errorf("Error %v in executing driver for %s", err, driverID)
		}
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

func saveWebhook(uuid string, name string, apiClient client.RancherClient) error {
	_, err := apiClient.GenericObject.Create(&client.GenericObject{
		Kind: "webhookToken",
		Name: name,
		Key:  uuid,
	})

	if err != nil {
		return fmt.Errorf("Failed to create webhook : %v", err)
	}
	return nil
}

func validateWebhook(uuid string, apiClient client.RancherClient) (int, error) {
	filters := make(map[string]interface{})
	filters["key"] = uuid
	filters["kind"] = "webhookToken"
	webhookCollection, err := apiClient.GenericObject.List(&client.ListOpts{
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
