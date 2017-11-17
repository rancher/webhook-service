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
	v1client "github.com/rancher/go-rancher/client"
	"github.com/rancher/go-rancher/v2"
	. "gopkg.in/check.v1"

	"github.com/rancher/webhook-service/drivers"
	"github.com/rancher/webhook-service/model"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type MySuite struct{}

var _ = Suite(&MySuite{})

func (s *MySuite) TestCreateAndUpgdateAndExcuteAndListAndDelete(c *C) {
	// Test creating a webhook
	constructURL := fmt.Sprintf("%s/v1-webhooks/receivers?projectId=1a1", server.URL)
	jsonStr := []byte(`{"driver":"forwardPost","name":"wh-name",
		"forwardPostConfig": {"projectId": "1a5","serviceName": "pipeline-server", "port": "60080", "path": "/v1"}}`)
	request, err := http.NewRequest("POST", constructURL, bytes.NewBuffer(jsonStr))
	c.Assert(err, IsNil)

	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler := HandleError(schemas, r.ConstructPayload)
	handler.ServeHTTP(response, request)
	c.Assert(response.Code, Equals, 200)

	resp, err := ioutil.ReadAll(response.Body)
	c.Assert(err, IsNil)

	wh := &model.Webhook{}
	err = json.Unmarshal(resp, wh)
	c.Assert(err, IsNil)
	c.Assert(wh.Name, Equals, "wh-name")
	c.Assert(wh.Driver, Equals, "forwardPost")
	c.Assert(wh.Id, Equals, "1")
	c.Assert(wh.URL, Not(Equals), "")
	c.Assert(wh.ForwardPostConfig.ProjectID, Equals, "1a5")
	c.Assert(wh.ForwardPostConfig.ServiceName, Equals, "pipeline-server")
	c.Assert(wh.ForwardPostConfig.Port, Equals, "60080")
	c.Assert(wh.ForwardPostConfig.Path, Equals, "/v1")
	c.Assert(wh.Links["self"], Matches, "*\\/v1-webhooks\\/receivers\\/1\\?projectId=1a1", Commentf("Bad self URL: %v", wh.Links["self"]))

	// Test getting the created webhook by id
	byID := fmt.Sprintf("%s/v1-webhooks/receivers/1?projectId=1a1", server.URL)
	request, err = http.NewRequest("GET", byID, nil)
	c.Assert(err, IsNil)

	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	c.Assert(response.Code, Equals, 200, Commentf("StatusCode %d means get failed", response.Code))

	resp, err = ioutil.ReadAll(response.Body)
	c.Assert(err, IsNil)

	wh = &model.Webhook{}
	err = json.Unmarshal(resp, wh)
	c.Assert(err, IsNil)
	c.Assert(wh.Name, Equals, "wh-name")
	c.Assert(wh.Driver, Equals, "forwardPost")
	c.Assert(wh.Id, Equals, "1")
	c.Assert(wh.URL, Not(Equals), "")
	c.Assert(wh.ForwardPostConfig.ProjectID, Equals, "1a5")
	c.Assert(wh.ForwardPostConfig.ServiceName, Equals, "pipeline-server")
	c.Assert(wh.ForwardPostConfig.Port, Equals, "60080")
	c.Assert(wh.ForwardPostConfig.Path, Equals, "/v1")

	// Test executing the webhook
	url := wh.URL
	requestExecute, err := http.NewRequest("POST", url, nil)
	c.Assert(err, IsNil)
	response = httptest.NewRecorder()
	handler = HandleError(schemas, r.Execute)
	handler.ServeHTTP(response, requestExecute)
	c.Assert(response.Code, Equals, 200, Commentf("StatusCode %d means get failed", response.Code))

	//List webhooks
	requestList, err := http.NewRequest("GET", constructURL, nil)
	c.Assert(err, IsNil)

	requestList.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, requestList)
	c.Assert(response.Code, Equals, 200, Commentf("StatusCode %d means get failed", response.Code))

	resp, err = ioutil.ReadAll(response.Body)
	c.Assert(err, IsNil)

	whCollection := &model.WebhookCollection{}
	err = json.Unmarshal(resp, whCollection)
	c.Assert(err, IsNil)
	c.Assert(whCollection.Data, HasLen, 1, Commentf("Added webhook not listed"))

	wh = &whCollection.Data[0]
	c.Assert(wh.Name, Equals, "wh-name")
	c.Assert(wh.Driver, Equals, "forwardPost")
	c.Assert(wh.Id, Equals, "1")
	c.Assert(wh.URL, Not(Equals), "")
	c.Assert(wh.ForwardPostConfig.ProjectID, Equals, "1a5")
	c.Assert(wh.ForwardPostConfig.ServiceName, Equals, "pipeline-server")
	c.Assert(wh.ForwardPostConfig.Port, Equals, "60080")
	c.Assert(wh.ForwardPostConfig.Path, Equals, "/v1")
	c.Assert(wh.Links["self"], Matches, "*\\/v1-webhooks\\/receivers\\/1\\?projectId=1a1", Commentf("Bad self URL: %v", wh.Links["self"]))

	//Delete
	request, err = http.NewRequest("DELETE", byID, nil)
	c.Assert(err, IsNil)

	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	c.Assert(response.Code, Equals, 204, Commentf("StatusCode %d means delete failed", response.Code))
}

type MockForwardPostDriver struct {
	expectedConfig model.ForwardPost
}

func (s *MockForwardPostDriver) Execute(conf interface{}, apiClient *client.RancherClient, request *http.Request) (int, error) {
	config := &model.ForwardPost{}

	if err := mapstructure.Decode(conf, config); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Couldn't unmarshal config: %v", err)
	}

	if config.ServiceName != s.expectedConfig.ServiceName {
		return 500, fmt.Errorf("Tag. Expected %v, Actual %v", s.expectedConfig.ServiceName, config.ServiceName)
	}
	logrus.Infof("Execute of mock upgradeService driver")
	return 0, nil
}

func (s *MockForwardPostDriver) ValidatePayload(conf interface{}, apiClient *client.RancherClient) (int, error) {
	if _, ok := conf.(model.ForwardPost); !ok {
		return http.StatusInternalServerError, fmt.Errorf("Can't process config")
	}

	logrus.Infof("Validate payload of mock forwardPost driver")
	return 0, nil
}

func (s *MockForwardPostDriver) GetDriverConfigResource() interface{} {
	return model.ForwardPost{}
}

func (s *MockForwardPostDriver) CustomizeSchema(schema *v1client.Schema) *v1client.Schema {
	return schema
}

func (s *MockForwardPostDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	ss := &drivers.ForwardPostDriver{}
	return ss.ConvertToConfigAndSetOnWebhook(conf, webhook)
}
