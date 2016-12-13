package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/rancher-auth-service/util"
	"github.com/rancher/webhook-service/drivers"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

var server *httptest.Server
var privateKeyFile string
var publicKeyFile string
var token string

type ServiceScalerTest struct{}

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
	return &client.WebhookCollection{}, nil
}

func (s *ServiceScalerTest) Execute(payload map[string]interface{}, apiClient client.RancherClient) (int, error) {
	logrus.Infof("Execute of mock driver")
	return 0, nil
}

func (s *ServiceScalerTest) ValidatePayload(input map[string]interface{}, apiClient client.RancherClient) (int, error) {
	logrus.Infof("Validate payload of mock driver")
	return 0, nil
}

func (s *ServiceScalerTest) GetSchema() interface{} {
	logrus.Infof("GetSchema of mock driver")
	return ServiceScalerTest{}
}

type ExecuteStructTest struct{}

func (e *ExecuteStructTest) GetClient(projectID string) (client.RancherClient, error) {
	logrus.Infof("RancherClientFactory GetClient")
	mockWebhook := &mockWebhook{}
	mockClient := &client.RancherClient{
		Webhook: mockWebhook,
	}
	return *mockClient, nil
}

func init() {
	PrivateKey = util.ParsePrivateKey("../testutils/private.pem")
	PublicKey = util.ParsePublicKey("../testutils/public.pem")
	drivers.Drivers = map[string]drivers.WebhookDriver{}
	drivers.Drivers["serviceScaleTest"] = &ServiceScalerTest{}
	server = httptest.NewServer(NewRouter())
}

func TestConstruct(t *testing.T) {
	constructURL := fmt.Sprintf("%s/v1-webhooks-generate", server.URL)
	jsonStr := []byte(`{"driver":"serviceScaleTest","name":"Test service scale_1"}`)
	request, err := http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-API-Project-Id", "1a1")

	response := httptest.NewRecorder()
	r := RouteHandler{}
	r.rcf = &ExecuteStructTest{}
	f := HandleError
	handler := f(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)

	if response.Code != 200 {
		t.Errorf("StatusCode %d means ConstructPayloadTest failed", response.Code)
	}
	resp, err := ioutil.ReadAll(response.Body)
	payload := make(map[string]interface{})
	json.Unmarshal(resp, &payload)
	url := payload["url"].(string)
	token = strings.Split(url, "=")[1]
}

func TestExecute(t *testing.T) {
	receiverURL := fmt.Sprintf("%s/v1-webhooks-receiver", server.URL)
	form := url.Values{}
	form.Add("token", token)
	request, err := http.NewRequest("POST", receiverURL, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	r := RouteHandler{}
	r.rcf = &ExecuteStructTest{}
	f := HandleError
	handler := f(schemas, r.Execute)
	handler.ServeHTTP(response, request)

	if response.Code == 500 {
		t.Errorf("StatusCode %d means Execute failed", response.Code)
	}
}
