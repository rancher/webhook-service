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

func TestWebhookCreateAndExecuteScaleHost(t *testing.T) {
	label := make(map[string]string)
	label["foo"] = "bar"
	// Test creating a webhook
	constructURL := fmt.Sprintf("%s/v1-webhooks/receivers?projectId=1a1", server.URL)
	jsonStr := []byte(`{"driver":"scaleHost","name":"wh-name",
		"scaleHostConfig": {"hostSelector": {"foo": "bar"}, "amount": 1, "action": "up", "min": 1, "max": 4, "deleteOption": "mostRecent"}}`)
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
	err = json.Unmarshal(resp, wh)
	if err != nil {
		t.Fatal(err)
	}
	if wh.Name != "wh-name" || wh.Driver != "scaleHost" || wh.Id != "1" || wh.URL == "" || wh.ScaleHostConfig.HostSelector["foo"] != label["foo"] ||
		wh.ScaleHostConfig.Action != "up" || wh.ScaleHostConfig.Amount != 1 || wh.ScaleHostConfig.Min != 1 ||
		wh.ScaleHostConfig.Max != 4 || wh.ScaleHostConfig.Type != "scaleHost" {
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
	if wh.Name != "wh-name" || wh.Driver != "scaleHost" || wh.Id != "1" || wh.URL == "" || wh.ScaleHostConfig.HostSelector["foo"] != label["foo"] ||
		wh.ScaleHostConfig.Action != "up" || wh.ScaleHostConfig.Amount != 1 || wh.ScaleHostConfig.Min != 1 ||
		wh.ScaleHostConfig.Max != 4 || wh.ScaleHostConfig.Type != "scaleHost" {
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
	if wh.Name != "wh-name" || wh.Driver != "scaleHost" || wh.Id != "1" || wh.URL == "" || wh.ScaleHostConfig.HostSelector["foo"] != label["foo"] ||
		wh.ScaleHostConfig.Action != "up" || wh.ScaleHostConfig.Amount != 1 || wh.ScaleHostConfig.Min != 1 ||
		wh.ScaleHostConfig.Max != 4 || wh.ScaleHostConfig.Type != "scaleHost" {
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

func TestWebhookCreateInvalidMinMaxActionScaleHost(t *testing.T) {
	constructURL := fmt.Sprintf("%s/v1-webhooks/receivers?projectId=1a1", server.URL)
	jsonStr := []byte(`{"driver":"scaleHost","name":"wh-name",
		"scaleHostConfig": {"hostSelector": {"foo": "bar"}, "amount": 1, "action": "up", "min": -1, "max": 4, "deleteOption": "mostRecent"}}`)
	request, err := http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler := HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code == 200 {
		t.Fatalf("Invalid min")
	}

	jsonStr = []byte(`{"driver":"scaleHost","name":"wh-name",
		"scaleHostConfig": {"hostSelector": {"foo": "bar"}, "amount": 1, "action": "up", "min": 1, "max": -4, "deleteOption": "mostRecent"}}`)
	request, err = http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler = HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code == 200 {
		t.Fatalf("Invalid max")
	}

	jsonStr = []byte(`{"driver":"scaleHost","name":"wh-name",
		"scaleHostConfig": {"hostSelector": {"foo": "bar"}, "amount": 1.5, "action": "up", "min": 1, "max": 4, "deleteOption": "mostRecent"}}`)
	request, err = http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler = HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatalf("Amount of type float is invalid")
	}

	jsonStr = []byte(`{"driver":"scaleHost","name":"wh-name",
		"scaleHostConfig": {"hostSelector": {"foo": "bar"}, "amount": 1, "action": "up", "min": 1.5, "max": 4, "deleteOption": "mostRecent"}}`)
	request, err = http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler = HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatalf("Min of type float is invalid")
	}

	jsonStr = []byte(`{"driver":"scaleHost","name":"wh-name",
		"scaleHostConfig": {"hostSelector": {"foo": "bar"}, "amount": 1, "action": "up", "min": 1, "max": 4.5, "deleteOption": "mostRecent"}}`)
	request, err = http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler = HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatalf("Max of type float is invalid")
	}

	jsonStr = []byte(`{"driver":"scaleHost","name":"wh-name",
		"scaleHostConfig": {"hostSelector": {"foo": "bar"}, "amount": 1, "action": "up", "min": 1, "max": 4, "deleteOption": "random"}}`)
	request, err = http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler = HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatalf("Invalid delete option")
	}

	jsonStr = []byte(`{"driver":"scaleHost","name":"wh-name",
		"scaleHostConfig": {"hostSelector": {"foo": "bar"}, "amount": 1, "action": "random", "min": 1, "max": 4, "deleteOption": "mostRecent"}}`)
	request, err = http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler = HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatalf("Invalid action")
	}
}

type MockHostDriver struct {
	expectedConfig model.ScaleHost
}

func (s *MockHostDriver) Execute(conf interface{}, apiClient *client.RancherClient, reqbody interface{}) (int, error) {
	config := &model.ScaleHost{}
	err := mapstructure.Decode(conf, config)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Couldn't unmarshal config: %v", err)
	}

	if config.HostSelector["foo"] != s.expectedConfig.HostSelector["foo"] {
		return 500, fmt.Errorf("HostSelector. Expected %v, Actual %v", s.expectedConfig.HostSelector, config.HostSelector)
	}

	if config.Action != s.expectedConfig.Action {
		return 500, fmt.Errorf("Action. Expected %v, Actual %v", s.expectedConfig.Action, config.Action)
	}

	if config.Amount != s.expectedConfig.Amount {
		return 500, fmt.Errorf("Amount. Expected %v, Actual %v", s.expectedConfig.Amount, config.Amount)
	}

	logrus.Infof("Execute of mock scale host driver")
	return 0, nil
}

func (s *MockHostDriver) ValidatePayload(conf interface{}, apiClient *client.RancherClient) (int, error) {
	config, ok := conf.(model.ScaleHost)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("Can't process config")
	}

	if config.Action != s.expectedConfig.Action {
		return 400, fmt.Errorf("Action. Expected %v, Actual %v", s.expectedConfig.Action, config.Action)
	}

	if config.Amount != s.expectedConfig.Amount {
		return 500, fmt.Errorf("Amount. Expected %v, Actual %v", s.expectedConfig.Amount, config.Amount)
	}

	if config.Min != s.expectedConfig.Min {
		return 500, fmt.Errorf("Min. Expected %v, Actual %v", s.expectedConfig.Min, config.Min)
	}

	if config.Max != s.expectedConfig.Max {
		return 500, fmt.Errorf("Max. Expected %v, Actual %v", s.expectedConfig.Max, config.Max)
	}

	if config.DeleteOption != s.expectedConfig.DeleteOption {
		return 400, fmt.Errorf("Delete option. Expected %v, Actual %v", s.expectedConfig.DeleteOption, config.DeleteOption)
	}

	if config.HostSelector["foo"] != s.expectedConfig.HostSelector["foo"] {
		return 500, fmt.Errorf("Selector Label. Expected %v, Actual %v", s.expectedConfig.HostSelector, config.HostSelector)
	}

	logrus.Infof("Validate payload of mock scale host driver")
	return 0, nil
}

func (s *MockHostDriver) GetDriverConfigResource() interface{} {
	return model.ScaleHost{}
}

func (s *MockHostDriver) CustomizeSchema(schema *v1client.Schema) *v1client.Schema {
	return schema
}

func (s *MockHostDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	ss := &drivers.ScaleHostDriver{}
	return ss.ConvertToConfigAndSetOnWebhook(conf, webhook)
}
