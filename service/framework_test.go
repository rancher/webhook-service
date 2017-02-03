package service

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
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

	expectedServiceConfig := model.ScaleService{
		ScaleAction: "up",
		ScaleChange: 1,
		ServiceID:   "id",
		Min:         1,
		Max:         4,
	}
	drivers.Drivers["scaleService"] = &MockServiceDriver{expectedConfig: expectedServiceConfig}

	ServiceSelector := make(map[string]string)
	ServiceSelector["foo"] = "bar"
	expectedUpgradeServiceConfig := model.ServiceUpgrade{
		ServiceSelector: ServiceSelector,
		Image:           "wh-image",
		Tag:             "wh-tag",
		BatchSize:       1,
		IntervalMillis:  2,
		StartFirst:      true,
	}
	drivers.Drivers["serviceUpgrade"] = &MockUpgradeServiceDriver{expectedConfig: expectedUpgradeServiceConfig}

	HostSelector := make(map[string]string)
	HostSelector["foo"] = "bar"
	expectedHostConfig := model.ScaleHost{
		Action:       "up",
		Amount:       1,
		HostSelector: HostSelector,
		Min:          1,
		Max:          4,
		DeleteOption: "mostRecent",
	}
	drivers.Drivers["scaleHost"] = &MockHostDriver{expectedConfig: expectedHostConfig}

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
	if response.Code != 400 && response.Header().Get("Content-Type") != "application/json" {
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
	if response.Code != 400 && response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Expected 400 response code because of missing Content-Type header, got: %v", response.Code)
	}
	resp, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	respMessage := string(resp)
	strings.Contains(respMessage, "application/json")
}

type MockRancherClientFactory struct {
	mw *mockGenericObject
}

func (e *MockRancherClientFactory) GetClient(projectID string) (*client.RancherClient, error) {
	logrus.Infof("RancherClientFactory GetClient")

	mockClient := &client.RancherClient{
		GenericObject: e.mw,
	}
	return mockClient, nil
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
