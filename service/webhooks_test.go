package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/mitchellh/mapstructure"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/rancher-auth-service/util"
	"github.com/rancher/webhook-service/drivers"
)

var server *httptest.Server

// TODO Refactor this test to use gocheck
func init() {
	drivers.Drivers = map[string]drivers.WebhookDriver{}

	expected := drivers.ScaleService{
		ScaleAction: "up",
		ScaleChange: 1,
		ServiceID:   "id",
	}
	drivers.Drivers["scaleService"] = &MockDriver{expectedConfig: expected}
	server = httptest.NewServer(NewRouter(nil, nil))
}

func TestWebhookFramework(t *testing.T) {
	constructURL := fmt.Sprintf("%s/v1-webhooks", server.URL)
	jsonStr := []byte(`{"driver":"scaleService","name":"Test service scale_1",
		"scaleServiceConfig": {"serviceId": "id", "amount": 1, "action": "up"}}`)
	request, err := http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-API-Project-Id", "1a1")

	response := httptest.NewRecorder()

	privateKey := util.ParsePrivateKey("../testutils/private.pem")
	publicKey := util.ParsePublicKey("../testutils/public.pem")
	r := RouteHandler{
		privateKey: privateKey,
		publicKey:  publicKey,
	}
	r.rcf = &MockRancherClientFactory{}
	handler := HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)

	if response.Code != 200 {
		t.Fatalf("StatusCode %d means ConstructPayloadTest failed", response.Code)
	}
	resp, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	payload := make(map[string]interface{})
	err = json.Unmarshal(resp, &payload)
	if err != nil {
		t.Fatal(err)
	}

	url := payload["url"].(string)

	request, err = http.NewRequest("POST", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	handler = HandleError(schemas, r.Execute)
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Errorf("StatusCode %d means execute failed", response.Code)
	}
}

type MockDriver struct {
	expectedConfig drivers.ScaleService
}

func (s *MockDriver) Execute(conf interface{}, apiClient client.RancherClient) (int, error) {
	config := &drivers.ScaleService{}
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

	logrus.Infof("Validate payload of mock driver")
	return 0, nil
}

func (s *MockDriver) ValidatePayload(conf interface{}, apiClient client.RancherClient) (int, error) {
	config, ok := conf.(drivers.ScaleService)
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

	logrus.Infof("Validate payload of mock driver")
	return 0, nil
}

func (s *MockDriver) GetSchema() interface{} {
	return drivers.ScaleService{}
}

type MockRancherClientFactory struct{}

func (e *MockRancherClientFactory) GetClient(projectID string) (client.RancherClient, error) {
	logrus.Infof("RancherClientFactory GetClient")
	mockWebhook := &mockWebhook{}
	mockClient := &client.RancherClient{
		Webhook: mockWebhook,
	}
	return *mockClient, nil
}

type mockWebhook struct {
	client.WebhookOperations
	webhook *client.Webhook
}

func (m *mockWebhook) Create(webhook *client.Webhook) (*client.Webhook, error) {
	m.webhook = webhook
	webhook.Links = make(map[string]string)
	webhook.Links["self"] = "self"
	return webhook, nil
}

func (m *mockWebhook) List(opts *client.ListOpts) (*client.WebhookCollection, error) {
	return &client.WebhookCollection{Data: []client.Webhook{{}}}, nil
}
