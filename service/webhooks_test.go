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
	"strings"
	"testing"
)

type ServiceScalerTest struct{}

var server *httptest.Server
var privateKeyFile string
var publicKeyFile string
var token string

func (s *ServiceScalerTest) Execute(payload map[string]interface{}, apiClient client.RancherClient) (int, error) {
	logrus.Infof("Execute of mock driver")
	return 0, nil
}

func (s *ServiceScalerTest) GetSchema() interface{} {
	logrus.Infof("GetSchema of mock driver")
	return ServiceScalerTest{}
}

type ExecuteStructTest struct{}

func (e *ExecuteStructTest) GetClient(projectID string) (client.RancherClient, error) {
	logrus.Infof("RancherClientFactory GetClient")
	return client.RancherClient{}, nil
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
	jsonStr := []byte(`{"driver":"serviceScaleTest"}`)
	request, err := http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-API-ACCOUNT-ID", "1a1")
	response, err := http.DefaultClient.Do(request)

	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 {
		t.Errorf("StatusCode %d means ConstructPayloadTest failed", response.StatusCode)
	}
	resp, err := ioutil.ReadAll(response.Body)
	payload := make(map[string]interface{})
	json.Unmarshal(resp, &payload)
	url := payload["url"].(string)
	token = strings.Split(url, "=")[1]
}

func TestExecute(t *testing.T) {
	receiverURL := fmt.Sprintf("%s/v1-webhooks-receiver", server.URL)
	jsonData := fmt.Sprintf(`{"token":"%s"}`, token)
	jsonStr := []byte(jsonData)
	request, err := http.NewRequest("POST", receiverURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r := RouteHandler{}
	r.rcf = &ExecuteStructTest{}
	f := HandleError
	handler := f(schemas, r.Execute)
	handler.ServeHTTP(response, request)

	if response.Code != 200 {
		t.Errorf("StatusCode %d means Execute failed", response.Code)
	}
}
