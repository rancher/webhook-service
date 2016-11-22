package service

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher/v2"
	"io/ioutil"
	"net/http"
	"time"
)

type WebhookRequest map[string]interface{}

func Construct(w http.ResponseWriter, r *http.Request) {
	var webhookRequestData WebhookRequest
	log.Infof("Construct Payload")
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Construct failed with error: %v", err)
		return
	}
	json.Unmarshal(bytes, &webhookRequestData)
	_, err = ConstructPayload(webhookRequestData)
}

// ConstructPayload accepts and sends request to driver
func ConstructPayload(parameters map[string]interface{}) (map[string]interface{}, error) {
	cattleUrl := "http://localhost:8080/v2-beta"
	webhookType := parameters["driver_type"]
	switch webhookType {
	case "service":
		serviceId := parameters["service_id"].(string)
		serviceAccountId := parameters["service_account_id"].(string)
		url := fmt.Sprintf("%s/projects/%s/services/%s", cattleUrl, serviceAccountId, serviceId)
		parameters["url"] = url
		return parameters, nil
	default:
		err := fmt.Errorf("Driver not recognized")
		return nil, err
	}
	return nil, nil
}

func Execute(w http.ResponseWriter, r *http.Request) {
	var payload WebhookRequest
	log.Infof("Execute")
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Construct failed with error: %v", err)
		return
	}
	json.Unmarshal(bytes, &payload)
	Process(payload)
}

//Process executes the driver
func Process(payload map[string]interface{}) error {
	webhookType := payload["driver_type"]
	switch webhookType {
	case "service":
		scaleAction := payload["scale_action"]
		switch scaleAction {
		case "scale_out":
			err := ScaleOut(payload)
			if err != nil {
				return err
			}
		case "scale_in":
			err := ScaleIn(payload)
			if err != nil {
				return err
			}
		default:
			err := fmt.Errorf("Incorrect scale action")
			return err
		}
	}
	return nil
}

func getService(apiClient *client.RancherClient, ID string) (*client.Service, error) {
	service, err := apiClient.Service.ById(ID)
	if err != nil {
		log.Errorf("Error : %v\n", err)
		return nil, err
	}
	return service, nil
}

func updateService(apiClient *client.RancherClient, service *client.Service, newScale int64) (*client.Service, error) {
	service, err := apiClient.Service.Update(service, client.Service{
		Scale:        newScale,
		CurrentScale: newScale,
	})
	if err != nil {
		log.Errorf("Error : %v\n", err)
		return nil, err
	}
	return nil, nil
}

func getClientAndScaleChange(payload map[string]interface{}) (*client.RancherClient, int64, error) {
	apiClient, err := client.NewRancherClient(&client.ClientOpts{
		Timeout: time.Second * 30,
		Url:     payload["url"].(string),
	})
	if err != nil {
		log.Errorf("Error : %v\n", err)
		return nil, 0, err
	}
	scaleChange := payload["scale_change"].(float64)

	return apiClient, int64(scaleChange), nil
}

func ScaleOut(payload map[string]interface{}) error {
	apiClient, scaleChange, err := getClientAndScaleChange(payload)
	if err != nil {
		log.Errorf("Error : %v\n", err)
		return err
	}
	serviceId := payload["service_id"].(string)
	service, err := getService(apiClient, serviceId)
	newScale := service.Scale + scaleChange
	if err != nil {
		log.Errorf("Error : %v\n", err)
		return err
	}
	updateService(apiClient, service, newScale)
	return nil
}

func ScaleIn(payload map[string]interface{}) error {
	apiClient, scaleChange, err := getClientAndScaleChange(payload)
	if err != nil {
		log.Errorf("Error : %v\n", err)
		return err
	}
	serviceId := payload["service_id"].(string)
	service, err := getService(apiClient, serviceId)
	newScale := service.Scale - scaleChange
	if err != nil {
		log.Errorf("Error : %v\n", err)
		return err
	}
	updateService(apiClient, service, newScale)
	return nil
}
