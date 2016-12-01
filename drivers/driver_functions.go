package drivers

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/rancher-auth-service/util"
	"net/http"
	"time"
)

//WebhookDriver interface for all drivers
type WebhookDriver interface {
	ConstructPayload(input map[string]interface{}) (map[string]interface{}, error)
	Execute(payload map[string]interface{}) (int, map[string]interface{}, error)
}

type ServiceScaler struct{}

func (s *ServiceScaler) ConstructPayload(input map[string]interface{}) (map[string]interface{}, error) {
	payload := make(map[string]interface{})
	jwt, err := util.CreateTokenWithPayload(input, PrivateKey)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating JWT\n")
	}
	payload["url"] = jwt
	return payload, nil
}

func (s *ServiceScaler) Execute(payload map[string]interface{}) (int, map[string]interface{}, error) {
	responseCode, response, err := ScaleService(payload)
	if err != nil {
		return responseCode, nil, errors.Wrap(err, "Error in executing ScaleService driver\n")
	}
	return responseCode, response, nil
}

//ScaleService is the driver for scale out/in
func ScaleService(payload map[string]interface{}) (int, map[string]interface{}, error) {
	var newScale int64
	if _, ok := payload["projectId"].(string); !ok {
		return http.StatusInternalServerError, nil, fmt.Errorf("AccountId not provided by server")
	}
	if _, ok := payload["serviceId"].(string); !ok {
		return http.StatusBadRequest, nil, fmt.Errorf("serviceId of type string not provided")
	}
	if _, ok := payload["scaleAction"].(string); !ok {
		return http.StatusBadRequest, nil, fmt.Errorf("scaleAction of type string not provided")
	}
	if _, ok := payload["scaleChange"].(float64); !ok {
		return http.StatusBadRequest, nil, fmt.Errorf("scaleChange of type float64 not provided")
	}
	projectID := payload["projectId"].(string)
	serviceID := payload["serviceId"].(string)
	scaleAction := payload["scaleAction"].(string)
	scaleChange := payload["scaleChange"].(float64)
	url := fmt.Sprintf("%s/accounts/%s", CattleURL, projectID)

	apiClient, err := client.NewRancherClient(&client.ClientOpts{
		Timeout:   time.Second * 30,
		Url:       url,
		AccessKey: accessKey,
		SecretKey: secretKey,
	})

	if err != nil {
		return http.StatusInternalServerError, nil, errors.Wrap(err, "Error in creating API client\n")
	}

	service, err := apiClient.Service.ById(serviceID)
	if err != nil {
		return http.StatusInternalServerError, nil, errors.Wrap(err, "Error in getService")
	}

	if scaleAction == "scaleOut" {
		newScale = service.Scale + int64(scaleChange)
	} else if scaleAction == "scaleIn" {
		newScale = service.Scale - int64(scaleChange)
	} else {
		return http.StatusBadRequest, nil, fmt.Errorf("ScaleAction not provided")
	}

	service, err = apiClient.Service.Update(service, client.Service{
		Scale:        newScale,
		CurrentScale: newScale,
	})
	if err != nil {
		return http.StatusInternalServerError, nil, errors.Wrap(err, "Error in updateService")
	}
	payload["newScale"] = newScale
	return http.StatusOK, payload, nil
}
