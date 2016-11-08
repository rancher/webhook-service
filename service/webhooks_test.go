package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/rancher/rancher-auth-service/util"
	"github.com/rancher/webhook-service/drivers"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

type ServiceScalerTest struct{}

var server *httptest.Server
var privateKeyFile string
var publicKeyFile string
var jwt string

func (s *ServiceScalerTest) ConstructPayload(input map[string]interface{}) (map[string]interface{}, error) {
	log.Infof("ConstructPayload of mock driver")

	payload := make(map[string]interface{})
	jwt, err := util.CreateTokenWithPayload(input, drivers.PrivateKey)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating JWT\n")
	}
	payload["url"] = jwt
	return payload, nil
}

func (s *ServiceScalerTest) Execute(payload map[string]interface{}) (int, map[string]interface{}, error) {
	log.Infof("Execute of mock driver")
	return 0, payload, nil
}

func init() {
	drivers.PrivateKey = util.ParsePrivateKey("../testutils/private.pem")
	drivers.PublicKey = util.ParsePublicKey("../testutils/public.pem")
	drivers.Drivers = map[string]drivers.WebhookDriver{}
	drivers.Drivers["serviceScaleTest"] = &ServiceScalerTest{}
	server = httptest.NewServer(NewRouter())
}

func TestConstruct(t *testing.T) {
	constructURL := fmt.Sprintf("%s/v1-webhooks-generate", server.URL)
	jsonStr := []byte(`{"driver":"serviceScaleTest"}`)
	request, err := http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)

	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 {
		t.Errorf("StatusCode %d means ConstructPayloadTest failed", response.StatusCode)
	}
	resp, err := ioutil.ReadAll(response.Body)
	var payload WebhookRequest
	json.Unmarshal(resp, &payload)
	jwt = payload["url"].(string)
}

func TestExecute(t *testing.T) {
	receiverURL := fmt.Sprintf("%s/v1-webhooks-receiver", server.URL)
	jsonData := fmt.Sprintf(`{"url":"%s"}`, jwt)
	jsonStr := []byte(jsonData)
	request, err := http.NewRequest("POST", receiverURL, bytes.NewBuffer(jsonStr))
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 {
		t.Errorf("StatusCode %d means Execute failed", response.StatusCode)
	}
}
