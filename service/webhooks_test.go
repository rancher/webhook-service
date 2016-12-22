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
	"github.com/gorilla/mux"
	"github.com/mitchellh/mapstructure"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/rancher-auth-service/util"
	"github.com/rancher/webhook-service/drivers"
	"github.com/rancher/webhook-service/model"
)

var server *httptest.Server
var router *mux.Router
var r *RouteHandler

// TODO Refactor this test to use gocheck
func init() {
	drivers.Drivers = map[string]drivers.WebhookDriver{}

	expected := model.ScaleService{
		ScaleAction: "up",
		ScaleChange: 1,
		ServiceID:   "id",
	}
	drivers.Drivers["scaleService"] = &MockDriver{expectedConfig: expected}

	privateKey := util.ParsePrivateKey("../testutils/private.pem")
	publicKey := util.ParsePublicKey("../testutils/public.pem")
	r = &RouteHandler{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}

	mockWebhook := &mockGenericObject{
		created: map[string]*client.GenericObject{},
	}
	r.ClientFactory = &MockRancherClientFactory{
		mw: mockWebhook,
	}
	router = NewRouter(r)
	server = httptest.NewServer(router)
}

func TestWebhookCreateAndExecute(t *testing.T) {
	// Test creating a webhook
	constructURL := fmt.Sprintf("%s/v1-webhooks/receivers?projectId=1a1", server.URL)
	jsonStr := []byte(`{"driver":"scaleService","name":"wh-name",
		"scaleServiceConfig": {"serviceId": "id", "amount": 1, "action": "up"}}`)
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
		wh.ScaleServiceConfig.ScaleAction != "up" || wh.ScaleServiceConfig.ScaleChange != 1 {
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
	if wh.Name != "wh-name" || wh.Driver != "scaleService" || wh.Id != "1" || wh.URL == "" || wh.ScaleServiceConfig.ServiceID != "id" ||
		wh.ScaleServiceConfig.ScaleAction != "up" || wh.ScaleServiceConfig.ScaleChange != 1 {
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
		wh.ScaleServiceConfig.ScaleAction != "up" || wh.ScaleServiceConfig.ScaleChange != 1 {
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
	if response.Code != 403 {
		fmt.Printf("response : %v\n", response)
		t.Fatal("Execute - Webhook not revoked after delete")
	}
}

func TestMissingProjectIdHeader(t *testing.T) {
	constructURL := fmt.Sprintf("%s/v1-webhooks", server.URL)
	request, err := http.NewRequest("POST", constructURL, bytes.NewBuffer([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler := HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatalf("Expected 400 response code because of missing projectId, got: %v", response.Code)
	}
	resp, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	respMessage := string(resp)
	strings.Contains(respMessage, "projectId")
}

func TestMissingContentTypeHeader(t *testing.T) {
	constructURL := fmt.Sprintf("%s/v1-webhooks?projectId=1a1", server.URL)
	request, err := http.NewRequest("POST", constructURL, bytes.NewBuffer([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler := HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatalf("Expected 400 response code because of missing Content-Type header, got: %v", response.Code)
	}
	resp, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	respMessage := string(resp)
	strings.Contains(respMessage, "application/json")
}

type MockDriver struct {
	expectedConfig model.ScaleService
}

func (s *MockDriver) Execute(conf interface{}, apiClient client.RancherClient) (int, error) {
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

	logrus.Infof("Validate payload of mock driver")
	return 0, nil
}

func (s *MockDriver) ValidatePayload(conf interface{}, apiClient client.RancherClient) (int, error) {
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

	logrus.Infof("Validate payload of mock driver")
	return 0, nil
}

func (s *MockDriver) GetSchema() interface{} {
	return model.ScaleService{}
}

func (s *MockDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	ss := &drivers.ScaleServiceDriver{}
	return ss.ConvertToConfigAndSetOnWebhook(conf, webhook)
}

type MockRancherClientFactory struct {
	mw *mockGenericObject
}

func (e *MockRancherClientFactory) GetClient(projectID string) (client.RancherClient, error) {
	logrus.Infof("RancherClientFactory GetClient")

	mockClient := &client.RancherClient{
		GenericObject: e.mw,
	}
	return *mockClient, nil
}

type mockGenericObject struct {
	client.GenericObjectOperations
	created map[string]*client.GenericObject
}

func (m *mockGenericObject) Create(webhook *client.GenericObject) (*client.GenericObject, error) {
	webhook.Links = make(map[string]string)
	webhook.Links["self"] = "self"
	webhook.Id = "1"
	m.created[webhook.Id] = webhook
	return webhook, nil
}

func (m *mockGenericObject) List(opts *client.ListOpts) (*client.GenericObjectCollection, error) {
	webhooks := []client.GenericObject{}
	for _, wh := range m.created {
		webhooks = append(webhooks, *wh)
	}
	return &client.GenericObjectCollection{Data: webhooks}, nil
}

func (m *mockGenericObject) ById(id string) (*client.GenericObject, error) {
	fmt.Printf("%v %#v\n\n", id, m.created)
	if wh, ok := m.created[id]; ok {
		return wh, nil
	}
	return nil, fmt.Errorf("Doesn't exist")
}

func (m *mockGenericObject) Delete(container *client.GenericObject) error {
	delete(m.created, container.Id)
	return nil
}
