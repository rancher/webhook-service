package drivers

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/v2"
	"net/http"
)

type ScaleService struct {
	Driver      string  `json:"driver,omitempty"`
	ServiceID   string  `json:"serviceId,omitempty"`
	ScaleChange float64 `json:"scaleChange,omitempty"`
	ScaleAction string  `json:"scaleAction,omitempty"`
}

func (s *ScaleService) Execute(payload map[string]interface{}, apiClient client.RancherClient) (int, error) {
	var newScale int64
	if _, ok := payload["serviceId"].(string); !ok {
		return http.StatusBadRequest, fmt.Errorf("serviceId of type string not provided")
	}
	if _, ok := payload["scaleAction"].(string); !ok {
		return http.StatusBadRequest, fmt.Errorf("scaleAction of type string not provided")
	}
	if _, ok := payload["scaleChange"].(float64); !ok {
		return http.StatusBadRequest, fmt.Errorf("scaleChange of type float64 not provided")
	}
	serviceID := payload["serviceId"].(string)
	scaleAction := payload["scaleAction"].(string)
	scaleChange := payload["scaleChange"].(float64)

	service, err := apiClient.Service.ById(serviceID)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Error in getService")
	}

	if scaleAction == "scaleOut" {
		newScale = service.Scale + int64(scaleChange)
	} else if scaleAction == "scaleIn" {
		newScale = service.Scale - int64(scaleChange)
	} else {
		return http.StatusBadRequest, fmt.Errorf("ScaleAction not provided")
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
