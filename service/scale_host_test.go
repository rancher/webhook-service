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
	"github.com/rancher/go-rancher/v3"
	"github.com/rancher/webhook-service/drivers"
	"github.com/rancher/webhook-service/model"
)

func TestWebhookCreateAndExecuteWithHostTemplateID(t *testing.T) {
	// Test creating a webhook
	constructURL := fmt.Sprintf("%s/v1-webhooks/receivers?projectId=1a1", server.URL)
	jsonStr := []byte(`{"driver": "scaleHost", "name": "wh-name",
	"scaleHostConfig": {"action": "up",	"amount": 1, "hostTemplateId": "1ht1", "min": 1, "max": 4, "deleteOption": "mostRecent"}}`)
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
	if wh.Name != "wh-name" || wh.Driver != "scaleHost" || wh.Id != "1" || wh.URL == "" ||
		wh.ScaleHostConfig.Action != "up" || wh.ScaleHostConfig.Amount != 1 || wh.ScaleHostConfig.Min != 1 ||
		wh.ScaleHostConfig.Max != 4 || wh.ScaleHostConfig.Type != "scaleHost" || wh.ScaleHostConfig.HostTemplateID != "1ht1" {
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
	if wh.Name != "wh-name" || wh.Driver != "scaleHost" || wh.Id != "1" || wh.URL == "" ||
		wh.ScaleHostConfig.Action != "up" || wh.ScaleHostConfig.Amount != 1 || wh.ScaleHostConfig.Min != 1 ||
		wh.ScaleHostConfig.Max != 4 || wh.ScaleHostConfig.Type != "scaleHost" || wh.ScaleHostConfig.HostTemplateID != "1ht1" {
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
	if wh.Name != "wh-name" || wh.Driver != "scaleHost" || wh.Id != "1" || wh.URL == "" ||
		wh.ScaleHostConfig.Action != "up" || wh.ScaleHostConfig.Amount != 1 || wh.ScaleHostConfig.Min != 1 ||
		wh.ScaleHostConfig.Max != 4 || wh.ScaleHostConfig.Type != "scaleHost" || wh.ScaleHostConfig.HostTemplateID != "1ht1" {
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

func TestWebhookCreateInvalidMinMaxActionWithHostTemplateID(t *testing.T) {
	constructURL := fmt.Sprintf("%s/v1-webhooks/receivers?projectId=1a1", server.URL)
	jsonStr := []byte(`{"driver":"scaleHost","name":"wh-name",
		"scaleHostConfig": {"action": "up", "amount": 1, "hostTemplateId": "1ht1",  "min": -1, "max": 4, "deleteOption": "mostRecent"}}`)
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
		"scaleHostConfig": {"action": "up", "amount": 1, "hostTemplateId": "1ht1",  "min": 1, "max": -4, "deleteOption": "mostRecent"}}`)
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
		"scaleHostConfig": {"action": "up", "amount": 1.5, "hostTemplateId": "1ht1",  "min": 1, "max": 4, "deleteOption": "mostRecent"}}`)
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
		"scaleHostConfig": {"action": "up", "amount": 1, "hostTemplateId": "1ht1",  "min": 1.5, "max": 4, "deleteOption": "mostRecent"}}`)
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
		"scaleHostConfig": {"action": "up", "amount": 1, "hostTemplateId": "1ht1",  "min": 1, "max": 4.5, "deleteOption": "mostRecent"}}`)
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
		"scaleHostConfig": {"action": "up", "amount": 1, "hostTemplateId": "1ht1",  "min": 1, "max": 4, "deleteOption": "random"}}`)
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
		"scaleHostConfig": {"action": "down", "amount": 1, "hostTemplateId": "1ht1",  "min": 1, "max": 4, "deleteOption": "mostRecent"}}`)
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

	jsonStr = []byte(`{"driver":"scaleHost","name":"wh-name",
		"scaleHostConfig": {"action": "down", "amount": 1, "hostTemplateId": "random",  "min": 1, "max": 4, "deleteOption": "mostRecent"}}`)
	request, err = http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler = HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatalf("Invalid hostTemplateId")
	}
}

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
	expectedConfigLabel        model.ScaleHost
	expectedConfigHostTemplate model.ScaleHost
}

func (s *MockHostDriver) Execute(conf interface{}, apiClient *client.RancherClient, reqbody interface{}) (int, error) {
	config := &model.ScaleHost{}
	err := mapstructure.Decode(conf, config)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Couldn't unmarshal config: %v", err)
	}

	if config.HostTemplateID != "" {
		if config.HostTemplateID != s.expectedConfigHostTemplate.HostTemplateID {
			return 500, fmt.Errorf("HostTemplateID. Expected %v, Actual %v", s.expectedConfigHostTemplate.HostTemplateID, config.HostTemplateID)
		}

		if config.Action != s.expectedConfigHostTemplate.Action {
			return 500, fmt.Errorf("Action. Expected %v, Actual %v", s.expectedConfigHostTemplate.Action, config.Action)
		}

		if config.Amount != s.expectedConfigHostTemplate.Amount {
			return 500, fmt.Errorf("Amount. Expected %v, Actual %v", s.expectedConfigHostTemplate.Amount, config.Amount)
		}

		logrus.Infof("Execute of mock scale host by HostTemplateID driver")
	} else {
		if config.HostSelector["foo"] != s.expectedConfigLabel.HostSelector["foo"] {
			return 500, fmt.Errorf("HostSelector. Expected %v, Actual %v", s.expectedConfigLabel.HostSelector, config.HostSelector)
		}

		if config.Action != s.expectedConfigLabel.Action {
			return 500, fmt.Errorf("Action. Expected %v, Actual %v", s.expectedConfigLabel.Action, config.Action)
		}

		if config.Amount != s.expectedConfigLabel.Amount {
			return 500, fmt.Errorf("Amount. Expected %v, Actual %v", s.expectedConfigLabel.Amount, config.Amount)
		}

		logrus.Infof("Execute of mock scale host with labels")
	}

	return 0, nil
}

func (s *MockHostDriver) ValidatePayload(conf interface{}, apiClient *client.RancherClient) (int, error) {
	config, ok := conf.(model.ScaleHost)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("Can't process config")
	}

	if config.HostTemplateID != "" {
		if config.Action != s.expectedConfigHostTemplate.Action {
			return 400, fmt.Errorf("Action. Expected %v, Actual %v", s.expectedConfigHostTemplate.Action, config.Action)
		}

		if config.Amount != s.expectedConfigHostTemplate.Amount {
			return 500, fmt.Errorf("Amount. Expected %v, Actual %v", s.expectedConfigHostTemplate.Amount, config.Amount)
		}

		if config.Min != s.expectedConfigHostTemplate.Min {
			return 500, fmt.Errorf("Min. Expected %v, Actual %v", s.expectedConfigHostTemplate.Min, config.Min)
		}

		if config.Max != s.expectedConfigHostTemplate.Max {
			return 500, fmt.Errorf("Max. Expected %v, Actual %v", s.expectedConfigHostTemplate.Max, config.Max)
		}

		if config.DeleteOption != s.expectedConfigHostTemplate.DeleteOption {
			return 400, fmt.Errorf("Delete option. Expected %v, Actual %v", s.expectedConfigHostTemplate.DeleteOption, config.DeleteOption)
		}

		if config.HostTemplateID != s.expectedConfigHostTemplate.HostTemplateID {
			return 500, fmt.Errorf("HostTemplateID. Expected %v, Actual %v", s.expectedConfigHostTemplate.HostTemplateID, config.HostTemplateID)
		}

		logrus.Infof("Validate payload of mock scale host by HostTemplateID driver")
	} else {
		if config.Action != s.expectedConfigLabel.Action {
			return 400, fmt.Errorf("Action. Expected %v, Actual %v", s.expectedConfigLabel.Action, config.Action)
		}

		if config.Amount != s.expectedConfigLabel.Amount {
			return 500, fmt.Errorf("Amount. Expected %v, Actual %v", s.expectedConfigLabel.Amount, config.Amount)
		}

		if config.Min != s.expectedConfigLabel.Min {
			return 500, fmt.Errorf("Min. Expected %v, Actual %v", s.expectedConfigLabel.Min, config.Min)
		}

		if config.Max != s.expectedConfigLabel.Max {
			return 500, fmt.Errorf("Max. Expected %v, Actual %v", s.expectedConfigLabel.Max, config.Max)
		}

		if config.DeleteOption != s.expectedConfigLabel.DeleteOption {
			return 400, fmt.Errorf("Delete option. Expected %v, Actual %v", s.expectedConfigLabel.DeleteOption, config.DeleteOption)
		}

		if config.HostSelector["foo"] != s.expectedConfigLabel.HostSelector["foo"] {
			return 500, fmt.Errorf("Selector Label. Expected %v, Actual %v", s.expectedConfigLabel.HostSelector, config.HostSelector)
		}

		logrus.Infof("Validate payload of mock scale host driver")
	}
	return 0, nil
}

func (s *MockHostDriver) GetDriverConfigResource() interface{} {
	return model.ScaleHost{}
}

func (s *MockHostDriver) CustomizeSchema(schema *client.Schema) *client.Schema {
	return schema
}

func (s *MockHostDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	ss := &drivers.ScaleHostDriver{}
	return ss.ConvertToConfigAndSetOnWebhook(conf, webhook)
}
