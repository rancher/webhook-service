package drivers

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	v1client "github.com/rancher/go-rancher/client"
	"github.com/rancher/go-rancher/v2"

	"github.com/rancher/webhook-service/config"
	"github.com/rancher/webhook-service/model"
)

type ForwardPostDriver struct {
}

func (s *ForwardPostDriver) ValidatePayload(conf interface{}, apiClient *client.RancherClient) (int, error) {
	if _, ok := conf.(model.ForwardPost); !ok {
		return http.StatusInternalServerError, fmt.Errorf("Can't process config")
	}
	return http.StatusOK, nil
}

func (s *ForwardPostDriver) Execute(conf interface{}, apiClient *client.RancherClient, request *http.Request) (int, error) {
	requestPayloadByte, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return 500, err
	}
	rancherConfig := config.GetConfig()
	webhookConfig := &model.ForwardPost{}
	if err = mapstructure.Decode(conf, webhookConfig); err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Couldn't unmarshal config")
	}

	arry := strings.Split(request.RequestURI, "?")
	CattleAddr := rancherConfig.CattleURL[:len(rancherConfig.CattleURL)-3]
	log.Debugf("Excute rancherConfig.CattleURL %v", CattleAddr)
	postURL := fmt.Sprintf("%s/r/projects/%s/%s:%s%s", CattleAddr, webhookConfig.ProjectID, webhookConfig.ServiceName, webhookConfig.Port, webhookConfig.Path)

	// append the query parameters to the postURL
	if arry[1] != "" {
		postURL += "?" + arry[1]
	}
	log.Debugf("Excute postURL %v", postURL)
	log.Debugf("Excute requestPayloadByte %v", requestPayloadByte)
	hopRequest, err := http.NewRequest("POST", postURL, bytes.NewBuffer(requestPayloadByte))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	client := &http.Client{}
	hopRequest.Header = request.Header
	hopRequest.SetBasicAuth(rancherConfig.CattleAccessKey, rancherConfig.CattleSecretKey)
	resp, err := client.Do(hopRequest)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	log.Debugf("Excute request %v", request)
	log.Debugf("Excute config %v", webhookConfig)

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return resp.StatusCode, errors.New(string(respBody))
	}
	log.Debugf("Response StatusCode: %v,Error: %v", resp.StatusCode, string(respBody))
	return resp.StatusCode, nil
}

func (s *ForwardPostDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	if upgradeConfig, ok := conf.(model.ForwardPost); ok {
		webhook.ForwardPostConfig = upgradeConfig
		webhook.ForwardPostConfig.Type = webhook.Driver
		return nil
	} else if configMap, ok := conf.(map[string]interface{}); ok {
		config := model.ForwardPost{}
		if err := mapstructure.Decode(configMap, &config); err != nil {
			return err
		}
		webhook.ForwardPostConfig = config
		webhook.ForwardPostConfig.Type = webhook.Driver
		return nil
	}
	return fmt.Errorf("Can't convert config %v", conf)
}

func (s *ForwardPostDriver) GetDriverConfigResource() interface{} {
	return model.ForwardPost{}
}

func (s *ForwardPostDriver) CustomizeSchema(schema *v1client.Schema) *v1client.Schema {
	return schema
}
