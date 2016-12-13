package drivers

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/v2"
	"net/http"
)

//ScaleService driver
type ScaleService struct {
	ServiceID   string  `json:"serviceId,omitempty"`
	ScaleChange float64 `json:"amount,omitempty"`
	ScaleAction string  `json:"action,omitempty"`
}

func (s *ScaleService) ValidatePayload(input map[string]interface{}, apiClient client.RancherClient) (int, error) {
	action, ok := input["action"].(string)
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("Scale action of type string not provided")
	}
	if action != "up" && action != "down" {
		return http.StatusBadRequest, fmt.Errorf("Invalid action for scaleService driver")
	}
	_, ok = input["amount"].(float64)
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("Scale amount of type float64 not provided")
	}
	serviceID, ok := input["serviceId"].(string)
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("serviceId of type string not provided")
	}
	service, err := apiClient.Service.ById(serviceID)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Error in getService")
	}

	if service == nil {
		return http.StatusBadRequest, fmt.Errorf("Requested service does not exist")
	}
	return http.StatusOK, nil
}

func (s *ScaleService) Execute(payload map[string]interface{}, apiClient client.RancherClient) (int, error) {
	var newScale int64

	serviceID := payload["serviceId"].(string)
	scaleAction := payload["action"].(string)
	scaleChange := payload["amount"].(float64)

	service, err := apiClient.Service.ById(serviceID)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Error in getService")
	}

	if scaleAction == "up" {
		newScale = service.Scale + int64(scaleChange)
	} else if scaleAction == "down" {
		newScale = service.Scale - int64(scaleChange)
	} else {
		return http.StatusBadRequest, fmt.Errorf("Scale action not provided")
	}

	service, err = apiClient.Service.Update(service, client.Service{
		Scale:        newScale,
		CurrentScale: newScale,
	})
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Error in updateService")
	}
	payload["newScale"] = newScale
	return http.StatusOK, nil
}

func (s *ScaleService) GetSchema() interface{} {
	return ScaleService{}
}
