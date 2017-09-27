package drivers

import (
	"bytes"
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

func (s *ServiceWebhookDriver) Execute(conf interface{}, apiClient *client.RancherClient, request interface{}) (int, error) {

	r := request.(*http.Request)
	requestPayloadByte, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return 500, err
	}
	rancherConfig := config.GetConfig()
	webhookConfig := &model.ServiceWebhook{}
	if err = mapstructure.Decode(conf, webhookConfig); err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Couldn't unmarshal config")
	}

	arry := strings.Split(r.RequestURI, "?")
	postURL := webhookConfig.ServiceURL
	if arry[1] != "" {
		postURL += "?" + arry[1]
	}
	log.Debugf("_____Excute postURL %v, ____", postURL)
	log.Debugf("_____Excute requestPayloadByte %v, ____", requestPayloadByte)
	hopRequest, err := http.NewRequest("POST", postURL, bytes.NewBuffer(requestPayloadByte))
	if err != nil {
		log.Errorf("fail:%v", err)
		return http.StatusInternalServerError, err
	}

	client := &http.Client{}
	hopRequest.Header = r.Header
	hopRequest.SetBasicAuth(rancherConfig.CattleAccessKey, rancherConfig.CattleSecretKey)
	resp, err := client.Do(hopRequest)
	if err != nil {
		log.Errorf("Error sending request to service:%v", err)
		return http.StatusInternalServerError, err
	}

	log.Debugf("_____Excute request %v, ____", request)
	log.Debugf("Excute config %v", webhookConfig)

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("read from response body error:%v", err)
		return http.StatusInternalServerError, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		log.Debugf("Response StatusCode: %v,Error: %v", resp.StatusCode, string(respBody))
		return resp.StatusCode, errors.New(string(respBody))
	}
	log.Debugf("Response StatusCode: %v,Error: %v", resp.StatusCode, string(respBody))
	return resp.StatusCode, nil
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
