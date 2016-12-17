package drivers

import (
	"fmt"
	"net/http"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/webhook-service/model"
)

type ScaleServiceDriver struct {
}

func (s *ScaleServiceDriver) ValidatePayload(conf interface{}, apiClient client.RancherClient) (int, error) {
	config, ok := conf.(model.ScaleService)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("Can't process config")
	}

	if config.ScaleAction == "" {
		return http.StatusBadRequest, fmt.Errorf("Scale action not provided")
	}

	if config.ScaleAction != "up" && config.ScaleAction != "down" {
		return http.StatusBadRequest, fmt.Errorf("Invalid action %v", config.ScaleAction)
	}

	if config.ScaleChange <= 0 {
		return http.StatusBadRequest, fmt.Errorf("Invalid amount: %v", config.ScaleChange)
	}

	if config.ServiceID == "" {
		return http.StatusBadRequest, fmt.Errorf("ServiceId not provided")
	}

	service, err := apiClient.Service.ById(config.ServiceID)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Error in getService")
	}

	if service == nil || service.Removed != "" {
		return http.StatusBadRequest, fmt.Errorf("Invalid service %v", config.ServiceID)
	}

	return http.StatusOK, nil
}

func (s *ScaleServiceDriver) Execute(conf interface{}, apiClient client.RancherClient) (int, error) {
	config := &model.ScaleService{}
	err := mapstructure.Decode(conf, config)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Couldn't unmarshal config")
	}
	var newScale int64
	serviceID := config.ServiceID
	scaleAction := config.ScaleAction
	scaleChange := config.ScaleChange

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
	return http.StatusOK, nil
}

func (s *ScaleServiceDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	if scaleConfig, ok := conf.(model.ScaleService); ok {
		webhook.ScaleServiceConfig = scaleConfig
		return nil
	} else if configMap, ok := conf.(map[string]interface{}); ok {
		config := model.ScaleService{}
		err := mapstructure.Decode(configMap, &config)
		if err != nil {
			return err
		}
		webhook.ScaleServiceConfig = config
		return nil
	}
	return fmt.Errorf("Can't convert config %v", conf)
}

func (s *ScaleServiceDriver) GetSchema() interface{} {
	return model.ScaleService{}
}
