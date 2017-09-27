package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/mitchellh/mapstructure"
	v1client "github.com/rancher/go-rancher/client"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/webhook-service/drivers"
	"github.com/rancher/webhook-service/model"
)

func TestCreateAndUpgdate(t *testing.T) {
	// Test creating a webhook
	constructURL := fmt.Sprintf("%s/v1-webhooks/receivers?projectId=1a1", server.URL)
	jsonStr := []byte(`{"driver":"serviceWebhook","name":"wh-name",
		"serviceWebhookConfig": {"serviceURL": "http"}}`)
	request, err := http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler := HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("StatusCode %d means ConstructPayloadTest failed", response.Code)
	}
	resp, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	wh := &model.Webhook{}

	if strings.Contains(string(resp), "\\u0026") {
		t.Fatalf("Bad execute URL, contains escaped `&` character")
	}

	err = json.Unmarshal(resp, wh)
	if err != nil {
		t.Fatal(err)
	}
	if wh.Name != "wh-name" || wh.Driver != "serviceWebhook" || wh.Id != "1" || wh.URL == "" || wh.ServiceWebhookConfig.ServiceURL != "http" {
		t.Fatalf("Unexpected webhook: %#v", wh)
	}
	if !strings.HasSuffix(wh.Links["self"], "/v1-webhooks/receivers/1?projectId=1a1") {
		t.Fatalf("Bad self URL: %v", wh.Links["self"])
	}

	// Test getting the created webhook by id
	byID := fmt.Sprintf("%s/v1-webhooks/receivers/1?projectId=1a1", server.URL)
	request, err = http.NewRequest("GET", byID, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("StatusCode %d means get failed", response.Code)
	}
	resp, err = ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	wh = &model.Webhook{}
	err = json.Unmarshal(resp, wh)
	if err != nil {
		t.Fatal(err)
	}
	if wh.Name != "wh-name" || wh.Driver != "serviceWebhook" || wh.Id != "1" || wh.URL == "" || wh.ServiceWebhookConfig.ServiceURL != "http" {
		t.Fatalf("Unexpected webhook: %#v", wh)
	}
	if !strings.HasSuffix(wh.Links["self"], "/v1-webhooks/receivers/1?projectId=1a1") {
		t.Fatalf("Bad self URL: %v", wh.Links["self"])
	}

	// Test executing the webhook
	url := wh.URL
	requestExecute, err := http.NewRequest("POST", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	handler = HandleError(schemas, r.Execute)
	handler.ServeHTTP(response, requestExecute)
	if response.Code != 200 {
		t.Errorf("StatusCode %d means execute failed", response.Code)
	}

	//List webhooks
	requestList, err := http.NewRequest("GET", constructURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	requestList.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, requestList)
	if response.Code != 200 {
		t.Fatalf("StatusCode %d means get failed", response.Code)
	}
	resp, err = ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	whCollection := &model.WebhookCollection{}
	err = json.Unmarshal(resp, whCollection)
	if err != nil {
		t.Fatal(err)
	}
	if len(whCollection.Data) != 1 {
		t.Fatal("Added webhook not listed")
	}
	wh = &whCollection.Data[0]
	if wh.Name != "wh-name" || wh.Driver != "serviceWebhook" || wh.Id != "1" || wh.URL == "" || wh.ServiceWebhookConfig.ServiceURL != "http" {
		t.Fatalf("Unexpected webhook: %#v", wh)
	}
	if !strings.HasSuffix(wh.Links["self"], "/v1-webhooks/receivers/1?projectId=1a1") {
		t.Fatalf("Bad self URL: %v", wh.Links["self"])
	}

	//Delete
	request, err = http.NewRequest("DELETE", byID, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != 204 {
		t.Fatalf("StatusCode %d means delete failed", response.Code)
	}
}

type MockServiceWebhookDriver struct {
	expectedConfig model.ServiceWebhook
}

func (s *MockServiceWebhookDriver) Execute(conf interface{}, apiClient *client.RancherClient, req interface{}) (int, error) {
	config := &model.ServiceWebhook{}
	request := make(map[string]interface{})
	err := mapstructure.Decode(conf, config)

	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Couldn't unmarshal config: %v", err)
	}
	request, ok := req.(map[string]interface{})
	if ok {
	}
	logrus.Infof("%v", request)
	if config.ServiceURL != s.expectedConfig.ServiceURL {
		return 500, fmt.Errorf("Tag. Expected %v, Actual %v", s.expectedConfig.ServiceURL, config.ServiceURL)
	}

	logrus.Infof("Execute of mock upgradeService driver")
	return 0, nil
}

func (s *MockServiceWebhookDriver) ValidatePayload(conf interface{}, apiClient *client.RancherClient) (int, error) {
	_, ok := conf.(model.ServiceWebhook)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("Can't process config")
	}

	logrus.Infof("Validate payload of mock serviceWebhook driver")
	return 0, nil
}

func (s *MockServiceWebhookDriver) GetDriverConfigResource() interface{} {
	return model.ServiceWebhook{}
}

func (s *MockServiceWebhookDriver) CustomizeSchema(schema *v1client.Schema) *v1client.Schema {
	return schema
}

func (s *MockServiceWebhookDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	ss := &drivers.ServiceWebhookDriver{}
	return ss.ConvertToConfigAndSetOnWebhook(conf, webhook)
}
