package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/mitchellh/mapstructure"
	"github.com/rancher/go-rancher/v3"
	"github.com/rancher/webhook-service/drivers"
	"github.com/rancher/webhook-service/model"
)

func TestWebhookCreateAndExecuteServiceUpgrade(t *testing.T) {
	label := make(map[string]string)
	label["foo"] = "bar"
	// Test creating a webhook
	constructURL := fmt.Sprintf("%s/v1-webhooks/receivers?projectId=1a1", server.URL)
	jsonStr := []byte(`{"driver":"serviceUpgrade","name":"wh-name",
		"serviceUpgradeConfig": {"serviceSelector": {"foo": "bar"}, "tag": "wh-tag", "batchSize": 1, "intervalMillis":2,
		"startFirst": true}}`)
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
	if wh.Name != "wh-name" || wh.Driver != "serviceUpgrade" || wh.Id != "1" || wh.URL == "" || wh.ServiceUpgradeConfig.ServiceSelector["foo"] != label["foo"] ||
		wh.ServiceUpgradeConfig.Tag != "wh-tag" || wh.ServiceUpgradeConfig.BatchSize != 1 ||
		wh.ServiceUpgradeConfig.IntervalMillis != 2 || wh.ServiceUpgradeConfig.StartFirst != true || wh.ServiceUpgradeConfig.Type != "serviceUpgrade" {
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
	if wh.Name != "wh-name" || wh.Driver != "serviceUpgrade" || wh.Id != "1" || wh.URL == "" || wh.ServiceUpgradeConfig.ServiceSelector["foo"] != label["foo"] ||
		wh.ServiceUpgradeConfig.Tag != "wh-tag" || wh.ServiceUpgradeConfig.BatchSize != 1 ||
		wh.ServiceUpgradeConfig.IntervalMillis != 2 || wh.ServiceUpgradeConfig.StartFirst != true || wh.ServiceUpgradeConfig.Type != "serviceUpgrade" {
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
	if wh.Name != "wh-name" || wh.Driver != "serviceUpgrade" || wh.Id != "1" || wh.URL == "" || wh.ServiceUpgradeConfig.ServiceSelector["foo"] != label["foo"] ||
		wh.ServiceUpgradeConfig.Tag != "wh-tag" || wh.ServiceUpgradeConfig.BatchSize != 1 ||
		wh.ServiceUpgradeConfig.IntervalMillis != 2 || wh.ServiceUpgradeConfig.StartFirst != true || wh.ServiceUpgradeConfig.Type != "serviceUpgrade" {
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

func TestWebhookTag(t *testing.T) {
	constructURL := fmt.Sprintf("%s/v1-webhooks/receivers?projectId=1a1", server.URL)
	testTagsPass := []string{
		"wh-tag",
		"1.2.3",
		"latest",
		"1.2.3-alpha1",
		"1.2.3-alpha1-pre1_rc1",
		"1234",
		"version",
		"VERSION",
		"versionLATEST123",
		"version.latest.123-version_latest",
		"_startsWithUnderscore"}

	testTagsFail := []string{
		"123456789123456789123456789123456789123456789123456789123456789123456789123456789123456789123456789123456789123456789123456789123456789",
		".startsWithPeriod",
		"-startsWithDash",
		"$%^startsWithSpecialCharacters",
		"Contains@#$characters",
		"endsWith^&*(",
		"1.2.3-alpha&"}

	for id, tag := range testTagsPass {
		name := tag + strconv.Itoa(id)
		jsonStr := []byte(`{"driver":"serviceUpgrade","name":"` + name + `",
			"serviceUpgradeConfig": {"serviceSelector": {"foo": "bar"}, "tag": "` + tag + `", "batchSize": 1, "intervalMillis":2,
			"startFirst": true}}`)
		request, err := http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler := HandleError(schemas, r.ConstructPayload)
		handler.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatalf("Tag is valid")
		}
		byID := fmt.Sprintf("%s/v1-webhooks/receivers/1?projectId=1a1", server.URL)
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

	for id, tag := range testTagsFail {
		name := tag + strconv.Itoa(id)
		jsonStr := []byte(`{"driver":"serviceUpgrade","name":"` + name + `",
			"serviceUpgradeConfig": {"serviceSelector": {"foo": "bar"}, "tag": "` + tag + `", "batchSize": 1, "intervalMillis":2,
			"startFirst": true}}`)
		request, err := http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler := HandleError(schemas, r.ConstructPayload)
		handler.ServeHTTP(response, request)
		if response.Code == 200 {
			fmt.Println(response.Body)
			t.Fatalf("Tag is invalid")
		}
	}
}

func TestWebhookInvalidBatchSizeInterval(t *testing.T) {
	constructURL := fmt.Sprintf("%s/v1-webhooks/receivers?projectId=1a1", server.URL)
	jsonStr := []byte(`{"driver":"serviceUpgrade","name":"wh-name",
		"serviceUpgradeConfig": {"serviceSelector": {"foo": "bar"}, "tag": "wh-tag", "batchSize": 0, "intervalMillis":2,
		"startFirst": true}}`)
	request, err := http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler := HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code == 200 {
		t.Fatalf("Invalid batchSize")
	}

	jsonStr = []byte(`{"driver":"serviceUpgrade","name":"wh-name",
		"serviceUpgradeConfig": {"serviceSelector": {"foo": "bar"}, "tag": "wh-tag", "batchSize": 1, "intervalMillis":0,
		"startFirst": true}}`)
	request, err = http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler = HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	if response.Code == 200 {
		t.Fatalf("Invalid intervalMillis")
	}
}

type MockUpgradeServiceDriver struct {
	expectedConfig model.ServiceUpgrade
}

func (s *MockUpgradeServiceDriver) Execute(conf interface{}, apiClient *client.RancherClient, payload interface{}) (int, error) {
	config := &model.ServiceUpgrade{}
	err := mapstructure.Decode(conf, config)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Couldn't unmarshal config: %v", err)
	}

	if config.ServiceSelector["foo"] != s.expectedConfig.ServiceSelector["foo"] {
		return 500, fmt.Errorf("ServiceSelector. Expected %v, Actual %v", s.expectedConfig.ServiceSelector, config.ServiceSelector)
	}

	if config.Tag != s.expectedConfig.Tag {
		return 500, fmt.Errorf("Tag. Expected %v, Actual %v", s.expectedConfig.Tag, config.Tag)
	}

	if config.BatchSize != s.expectedConfig.BatchSize {
		return 500, fmt.Errorf("BatchSize. Expected %v, Actual %v", s.expectedConfig.BatchSize, config.BatchSize)
	}

	if config.IntervalMillis != s.expectedConfig.IntervalMillis {
		return 500, fmt.Errorf("IntervalMillis. Expected %v, Actual %v", s.expectedConfig.IntervalMillis, config.IntervalMillis)
	}

	if config.StartFirst != s.expectedConfig.StartFirst {
		return 500, fmt.Errorf("StartFirst. Expected %v, Actual %v", s.expectedConfig.StartFirst, config.StartFirst)
	}

	logrus.Infof("Execute of mock upgradeService driver")
	return 0, nil
}

func (s *MockUpgradeServiceDriver) ValidatePayload(conf interface{}, apiClient *client.RancherClient) (int, error) {
	config, ok := conf.(model.ServiceUpgrade)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("Can't process config")
	}

	err := drivers.IsValidTag(config.Tag)
	if err != nil {
		return 400, err
	}

	if config.BatchSize != s.expectedConfig.BatchSize {
		return 500, fmt.Errorf("BatchSize. Expected %v, Actual %v", s.expectedConfig.BatchSize, config.BatchSize)
	}

	if config.IntervalMillis != s.expectedConfig.IntervalMillis {
		return 500, fmt.Errorf("IntervalMillis. Expected %v, Actual %v", s.expectedConfig.IntervalMillis, config.IntervalMillis)
	}

	if config.StartFirst != s.expectedConfig.StartFirst {
		return 500, fmt.Errorf("StartFirst. Expected %v, Actual %v", s.expectedConfig.StartFirst, config.StartFirst)
	}

	logrus.Infof("Validate payload of mock upgradeService driver")
	return 0, nil
}

func (s *MockUpgradeServiceDriver) GetDriverConfigResource() interface{} {
	return model.ServiceUpgrade{}
}

func (s *MockUpgradeServiceDriver) CustomizeSchema(schema *client.Schema) *client.Schema {
	return schema
}

func (s *MockUpgradeServiceDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	ss := &drivers.ServiceUpgradeDriver{}
	return ss.ConvertToConfigAndSetOnWebhook(conf, webhook)
}
