package drivers

import (
	"fmt"
	"net/http"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	v1client "github.com/rancher/go-rancher/client"
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

	if config.Min <= 0 {
		return http.StatusBadRequest, fmt.Errorf("Minimum scale not provided/invalid")
	}

	if config.Max <= 0 {
		return http.StatusBadRequest, fmt.Errorf("Maximum scale not provided/invalid")
	}

	service, err := apiClient.Service.ById(config.ServiceID)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Error in getService")
	}

	if service == nil || service.Removed != "" {
		return http.StatusBadRequest, fmt.Errorf("Invalid service %v", config.ServiceID)
	}

	if service.Kind != "service" {
		return http.StatusBadRequest, fmt.Errorf("Can only create webhooks for Services. The supplied service is of type %v", service.Kind)
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
	min := int64(config.Min)
	max := int64(config.Max)

	service, err := apiClient.Service.ById(serviceID)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Error in getService")
	}

	if service == nil || service.Removed != "" {
		return http.StatusBadRequest, fmt.Errorf("Service %v has been deleted", config.ServiceID)
	}

	if scaleAction == "up" {
		newScale = service.Scale + int64(scaleChange)
		if newScale > max {
			return http.StatusBadRequest, fmt.Errorf("Cannot scale above provided max scale value")
		}
	} else if scaleAction == "down" {
		if service.Scale-int64(scaleChange) <= 0 {
			return http.StatusBadRequest, fmt.Errorf("Cannot have negative scale")
		}
		newScale = service.Scale - int64(scaleChange)
		if newScale < min {
			return http.StatusBadRequest, fmt.Errorf("Cannot scale below provided min scale value")
		}
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

func (s *ScaleServiceDriver) GetDriverConfigResource() interface{} {
	return model.ScaleService{}
}

func (s *ScaleServiceDriver) CustomizeSchema(schema *v1client.Schema) *v1client.Schema {
	options := []string{"up", "down"}
	minValue := int64(1)

	action := schema.ResourceFields["action"]
	action.Type = "enum"
	action.Options = options
	schema.ResourceFields["action"] = action

	min := schema.ResourceFields["min"]
	min.Default = 1
	min.Min = &minValue
	schema.ResourceFields["min"] = min

	max := schema.ResourceFields["max"]
	max.Default = 100
	max.Min = &minValue
	schema.ResourceFields["max"] = max

	return schema
}
