package drivers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	// log "github.com/Sirupsen/logrus"
	log "github.com/Sirupsen/logrus"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	// "github.com/pkg/errors"
	v1client "github.com/rancher/go-rancher/client"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/webhook-service/config"
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

func (s *ServiceWebhookDriver) Execute(conf interface{}, apiClient *client.RancherClient, requestPayload interface{}, req interface{}) (int, error) {
	rancherConfig := config.GetConfig()
	webhookConfig := &model.ServiceWebhook{}
	err := mapstructure.Decode(conf, webhookConfig)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Couldn't unmarshal config")
	}
	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("Body should be of type map[string]interface{}")
	}
	originRequest := req.(*http.Request)

	arry := strings.Split(originRequest.RequestURI, "?")
	postURL := webhookConfig.ServiceURL
	if arry[1] != "" {
		postURL += "?" + arry[1]
	}
	log.Debugf("_____Excute postURL %v, ____", postURL)
	request, err := http.NewRequest("POST", postURL, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Errorf("fail:%v", err)
		return 500, err
	}

	client := &http.Client{}
	request.Header = originRequest.Header
	request.SetBasicAuth(rancherConfig.CattleAccessKey, rancherConfig.CattleSecretKey)
	rep, err := client.Do(request)
	if err != nil {
		log.Errorf("fail:%v", err)
		return 500, err
	}

	log.Debugf("_____Excute request %v, ____", request)
	log.Debugf("_____Excute paylod %v, ____", requestBody)
	log.Debugf("Excute config %v", webhookConfig)

	respBody, err := ioutil.ReadAll(rep.Body)
	if err != nil {
		log.Errorf("get response from service error:%v", err)
		return 500, err
	}
	if rep.StatusCode != 200 {
		log.Errorf("Excute error resp %v,%v", rep.StatusCode, string(respBody))
		return rep.StatusCode, errors.New(string(respBody))
	}
	log.Debugf("Excute resp %v", string(respBody))
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
