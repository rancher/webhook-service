package drivers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	// log "github.com/Sirupsen/logrus"
	log "github.com/Sirupsen/logrus"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	// "github.com/pkg/errors"
	v1client "github.com/rancher/go-rancher/client"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/webhook-service/model"
)

type ServiceWebhookDriver struct {
}

func (s *ServiceWebhookDriver) ValidatePayload(conf interface{}, apiClient *client.RancherClient) (int, error) {
	_, ok := conf.(model.ServiceWebhook)
	log.Infof("create Valid")
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("Can't process config")
	}
	return http.StatusOK, nil
}

func (s *ServiceWebhookDriver) Execute(conf interface{}, apiClient *client.RancherClient, requestPayload interface{}, requestHeader interface{}) (int, error) {
	config := &model.ServiceWebhook{}
	err := mapstructure.Decode(conf, config)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Couldn't unmarshal config")
	}
	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("Body should be of type map[string]interface{}")
	}
	requestHead := requestHeader.(http.Header)
	request, err := http.NewRequest("POST", config.ServiceURL, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Errorf("fail:%v", err)
		return 500, err
	}
	client := &http.Client{}
	request.Header = requestHead
	rep, err := client.Do(request)
	if err != nil {
		log.Errorf("fail:%v", err)
		return 500, err
	}

	log.Infof("_____Excute paylod %v, ____", requestHead)
	log.Infof("_____Excute paylod %v, ____", requestBody)
	log.Infof("Excute config %v", config)

	if rep.StatusCode != 200 {
		respBody, err := ioutil.ReadAll(rep.Body)
		if err != nil {
			log.Errorf("get response from service error:%v", err)
			return 500, err
		}

		return rep.StatusCode, errors.New(string(respBody))
	}

	defer rep.Body.Close()
	return 200, nil
}

func (s *ServiceWebhookDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	if upgradeConfig, ok := conf.(model.ServiceWebhook); ok {
		webhook.ServiceWebhookConfig = upgradeConfig
		webhook.ServiceWebhookConfig.Type = webhook.Driver
		return nil
	} else if configMap, ok := conf.(map[string]interface{}); ok {
		config := model.ServiceWebhook{}
		err := mapstructure.Decode(configMap, &config)
		if err != nil {
			return err
		}
		webhook.ServiceWebhookConfig = config
		webhook.ServiceWebhookConfig.Type = webhook.Driver
		return nil
	}
	return fmt.Errorf("Can't convert config %v", conf)
}

func (s *ServiceWebhookDriver) GetDriverConfigResource() interface{} {
	return model.ServiceWebhook{}
}

func (s *ServiceWebhookDriver) CustomizeSchema(schema *v1client.Schema) *v1client.Schema {
	return schema
}
