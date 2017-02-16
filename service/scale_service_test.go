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

func TestWebhookCreateAndExecuteScaleService(t *testing.T) {
	// Test creating a webhook
	constructURL := fmt.Sprintf("%s/v1-webhooks/receivers?projectId=1a1", server.URL)
	jsonStr := []byte(`{"driver":"scaleService","name":"wh-name",
		"scaleServiceConfig": {"serviceId": "id", "amount": 1, "action": "up", "min": 1, "max": 4}}`)
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
	if wh.Name != "wh-name" || wh.Driver != "scaleService" || wh.Id != "1" || wh.URL == "" || wh.ScaleServiceConfig.ServiceID != "id" ||
		wh.ScaleServiceConfig.ScaleAction != "up" || wh.ScaleServiceConfig.ScaleChange != 1 || wh.ScaleServiceConfig.Min != 1 ||
		wh.ScaleServiceConfig.Max != 4 || wh.ScaleServiceConfig.Type != "scaleService" {
		t.Fatalf("Unexpected webhook: %#v", wh)
	}
	if !strings.HasSuffix(wh.Links["self"], "/v1-webhooks/receivers/1?projectId=1a1") {
		t.Fatalf("Bad self URL: %v", wh.Links["self"])
	}

	// Test creating webhook with same name
	response = httptest.NewRecorder()
	handler = HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code != 400 && response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Duplicate name webhook creation should not be allowed")
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
	if wh.Name != "wh-name" || wh.Driver != "scaleService" || wh.Id != "1" || wh.URL == "" || wh.ScaleServiceConfig.ServiceID != "id" ||
		wh.ScaleServiceConfig.ScaleAction != "up" || wh.ScaleServiceConfig.ScaleChange != 1 || wh.ScaleServiceConfig.Min != 1 ||
		wh.ScaleServiceConfig.Max != 4 || wh.ScaleServiceConfig.Type != "scaleService" {
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

	// //List webhooks
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
	if wh.Name != "wh-name" || wh.Driver != "scaleService" || wh.Id != "1" || wh.URL == "" || wh.ScaleServiceConfig.ServiceID != "id" ||
		wh.ScaleServiceConfig.ScaleAction != "up" || wh.ScaleServiceConfig.ScaleChange != 1 || wh.ScaleServiceConfig.Min != 1 ||
		wh.ScaleServiceConfig.Max != 4 || wh.ScaleServiceConfig.Type != "scaleService" {
		t.Fatalf("Unexpected webhook: %#v", wh)
	}
	if !strings.HasSuffix(wh.Links["self"], "/v1-webhooks/receivers/1?projectId=1a1") {
		t.Fatalf("Bad self URL: %v", wh.Links["self"])
	}

	// //Delete
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

	//List after delete
	response = httptest.NewRecorder()
	router.ServeHTTP(response, requestList)
	if response.Code != 200 {
		t.Fatalf("StatusCode %d means get failed", response.Code)
	}
	resp, err = ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	whCollection = &model.WebhookCollection{}
	err = json.Unmarshal(resp, whCollection)
	if err != nil {
		t.Fatal(err)
	}
	if len(whCollection.Data) != 0 {
		t.Fatal("List - Webhook not revoked on delete")
	}

	// //Execute deleted
	response = httptest.NewRecorder()
	handler = HandleError(schemas, r.Execute)
	handler.ServeHTTP(response, requestExecute)
	if response.Code != 403 && response.Header().Get("Content-Type") != "application/json" {
		fmt.Printf("response : %v\n", response)
		t.Fatal("Execute - Webhook not revoked after delete")
	}
}

func TestWebhookInvalidMinMaxActionScaleService(t *testing.T) {
	constructURL := fmt.Sprintf("%s/v1-webhooks/receivers?projectId=1a1", server.URL)
	jsonStr := []byte(`{"driver":"scaleService","name":"wh-name",
		"scaleServiceConfig": {"serviceId": "id", "amount": 1, "action": "up", "min": -1, "max": 4}}`)
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

	jsonStr = []byte(`{"driver":"scaleService","name":"wh-name",
		"scaleServiceConfig": {"serviceId": "id", "amount": 1, "action": "up", "min": 1, "max": -4}}`)
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

	jsonStr = []byte(`{"driver":"scaleService","name":"wh-name",
		"scaleServiceConfig": {"serviceId": "id", "amount": 1.5, "action": "up", "min": 1, "max": 4}}`)
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

	jsonStr = []byte(`{"driver":"scaleService","name":"wh-name",
		"scaleServiceConfig": {"serviceId": "id", "amount": 1, "action": "up", "min": 1.5, "max": 4}}`)
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

	jsonStr = []byte(`{"driver":"scaleService","name":"wh-name",
		"scaleServiceConfig": {"serviceId": "id", "amount": 1, "action": "up", "min": 1, "max": 4.5}}`)
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
}

func TestCreateWithInvalidDriver(t *testing.T) {
	constructURL := fmt.Sprintf("%s/v1-webhooks/receivers?projectId=1a1", server.URL)
	jsonStr := []byte(`{"driver":"driverInvalid","name":"wh-name",
		"scaleServiceConfig": {"serviceId": "id", "amount": 1, "action": "up", "min": -1, "max": 4}}`)
	request, err := http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler := HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatalf("Invalid driverservice/execute_handler.go")
	}
}

type MockServiceDriver struct {
	expectedConfig model.ScaleService
}

func (s *MockServiceDriver) Execute(conf interface{}, apiClient *client.RancherClient, payload interface{}) (int, error) {
	config := &model.ScaleService{}
	err := mapstructure.Decode(conf, config)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Couldn't unmarshal config: %v", err)
	}

	if config.ServiceID != s.expectedConfig.ServiceID {
		return 500, fmt.Errorf("ServiceID. Expected %v, Actual %v", s.expectedConfig.ServiceID, config.ServiceID)
	}

	if config.ScaleAction != s.expectedConfig.ScaleAction {
		return 500, fmt.Errorf("ServiceAction. Expected %v, Actual %v", s.expectedConfig.ScaleAction, config.ScaleAction)
	}

	if config.ScaleChange != s.expectedConfig.ScaleChange {
		return 500, fmt.Errorf("ServiceChange. Expected %v, Actual %v", s.expectedConfig.ScaleChange, config.ScaleChange)
	}

	logrus.Infof("Execute of mock scaleService driver")
	return 0, nil
}

func (s *MockServiceDriver) ValidatePayload(conf interface{}, apiClient *client.RancherClient) (int, error) {
	config, ok := conf.(model.ScaleService)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("Can't process config")
	}

	if config.ServiceID != s.expectedConfig.ServiceID {
		return 500, fmt.Errorf("ServiceID. Expected %v, Actual %v", s.expectedConfig.ServiceID, config.ServiceID)
	}

	if config.ScaleAction != s.expectedConfig.ScaleAction {
		return 500, fmt.Errorf("ServiceAction. Expected %v, Actual %v", s.expectedConfig.ScaleAction, config.ScaleAction)
	}

	if config.ScaleChange != s.expectedConfig.ScaleChange {
		return 500, fmt.Errorf("ServiceChange. Expected %v, Actual %v", s.expectedConfig.ScaleChange, config.ScaleChange)
	}

	if config.Min != s.expectedConfig.Min {
		return 500, fmt.Errorf("Min. Expected %v, Actual %v", s.expectedConfig.Min, config.Min)
	}

	if config.Max != s.expectedConfig.Max {
		return 500, fmt.Errorf("Max. Expected %v, Actual %v", s.expectedConfig.Max, config.Max)
	}

	logrus.Infof("Validate payload of mock scaleService driver")
	return 0, nil
}

func (s *MockServiceDriver) GetDriverConfigResource() interface{} {
	return model.ScaleService{}
}

func (s *MockServiceDriver) CustomizeSchema(schema *v1client.Schema) *v1client.Schema {
	return schema
}

func (s *MockServiceDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	ss := &drivers.ScaleServiceDriver{}
	return ss.ConvertToConfigAndSetOnWebhook(conf, webhook)
}
