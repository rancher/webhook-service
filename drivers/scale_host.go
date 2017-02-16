package drivers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	v1client "github.com/rancher/go-rancher/client"
	"github.com/rancher/go-rancher/v2"
	rConfig "github.com/rancher/webhook-service/config"
	"github.com/rancher/webhook-service/model"
)

type ScaleHostDriver struct {
}

func (s *ScaleHostDriver) ValidatePayload(conf interface{}, apiClient client.RancherClient) (int, error) {
	config, ok := conf.(model.ScaleHost)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("Can't process config")
	}

	if config.Action == "" {
		return http.StatusBadRequest, fmt.Errorf("Scale action not provided")
	}

	if config.Action != "up" && config.Action != "down" {
		return http.StatusBadRequest, fmt.Errorf("Invalid action %v", config.Action)
	}

	if config.Amount <= 0 {
		return http.StatusBadRequest, fmt.Errorf("Invalid amount: %v", config.Amount)
	}

	if config.HostID == "" {
		return http.StatusBadRequest, fmt.Errorf("HostID not provided")
	}

	if config.Min <= 0 {
		return http.StatusBadRequest, fmt.Errorf("Minimum scale not provided/invalid")
	}

	if config.Max <= 0 {
		return http.StatusBadRequest, fmt.Errorf("Maximum scale not provided/invalid")
	}

	if config.Min >= config.Max {
		return http.StatusBadRequest, fmt.Errorf("Max must be greater than min")
	}

	host, err := apiClient.Host.ById(config.HostID)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Error in getting Host")
	}

	if host == nil || host.Removed != "" {
		return http.StatusBadRequest, fmt.Errorf("Invalid host %v", config.HostID)
	}

	return http.StatusOK, nil
}

func (s *ScaleHostDriver) Execute(conf interface{}, apiClient client.RancherClient) (int, error) {
	var currNameSuffix, baseHostName, currCloneName, suffix string
	var count int64
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	config := &model.ScaleHost{}
	err := mapstructure.Decode(conf, config)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Couldn't unmarshal config")
	}

	hostID := config.HostID
	action := config.Action
	amount := config.Amount
	// min := config.Min
	// max := config.Max

	// http GET to get host informatio
	cattleConfig := rConfig.GetConfig()
	cattleURL := cattleConfig.CattleURL
	u, err := url.Parse(cattleURL)
	if err != nil {
		panic(err)
	}
	cattleURL = strings.Split(cattleURL, u.Path)[0] + "/v2-beta"
	hostURL := cattleURL + "/hosts/" + hostID

	host, err := getHosts(hostURL)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	hostCreateURL := cattleURL + "/projects/" + host["accountId"].(string) + "/hosts"

	// Remove domain from host name, scaleHost12.foo.com becomes scaleHost12
	// Remove largest number suffix from end, scaleHost12 becomes scaleHost
	// Name has precedence over hostname. If name is set, empty this field for the clones
	if host["name"] != nil {
		baseHostName = host["name"].(string)
		host["name"] = nil
	} else {
		baseHostName = host["hostname"].(string)
	}
	baseHostName = strings.Split(baseHostName, ".")[0]
	re := regexp.MustCompile("[0-9]+$")
	baseSuffix := re.FindString(baseHostName)
	basePrefix := strings.TrimRight(baseHostName, baseSuffix)

	fieldsToRemove := []string{"id", "uuid", "actions", "links"}
	for _, field := range fieldsToRemove {
		delete(host, field)
	}

	// Get all hosts created after base host
	hostGetURL := hostCreateURL + "?sort=created&order=desc"
	hostsResp, err := getHosts(hostGetURL)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	allHosts := hostsResp["data"].([]interface{})

	//The first host with name like baseHostName has the largest suffix
	count = 0
	deleteHosts := []map[string]interface{}{}
	for _, val := range allHosts {
		currentHost := val.(map[string]interface{})
		if currentHost["name"] != nil {
			currCloneName = currentHost["name"].(string)
		} else {
			currCloneName = currentHost["hostname"].(string)
		}

		if !strings.Contains(currCloneName, basePrefix) {
			continue
		}

		if action == "down" {
			deleteHosts = append(deleteHosts, currentHost)
			count++
			if count == amount {
				break
			}
			continue
		}

		currCloneName = strings.Split(currCloneName, ".")[0]
		re := regexp.MustCompile("[0-9]+$")
		suffix = re.FindString(currCloneName)

		break
	}

	if action == "up" {
		count = 0
		for count < amount {
			if suffix != "" {
				prevNumber, err := strconv.Atoi(suffix)
				if err != nil {
					return http.StatusInternalServerError, fmt.Errorf("Error converting %s to int : %v", suffix, err)
				}
				currNumber := prevNumber + 1
				currNameSuffix = leftPad(strconv.Itoa(currNumber), "0", len(suffix))
			} else {
				currNameSuffix = "-2"
			}

			name := basePrefix + currNameSuffix
			host["hostname"] = name

			code, err := createHost(host, hostCreateURL, httpClient)
			if err != nil {
				return code, err
			}

			suffix = currNameSuffix
			count++
		}
	} else if action == "down" {
		for _, lastClone := range deleteHosts {
			if lastClone["id"].(string) == hostID {
				return http.StatusBadRequest, fmt.Errorf("Cannot delete base host")
			}

			deactivateURL := hostCreateURL + "/" + lastClone["id"].(string) + "?action=deactivate"
			_, err = http.Post(deactivateURL, "", bytes.NewReader([]byte{}))
			if err != nil {
				return http.StatusInternalServerError, fmt.Errorf("Error deactivating host : %v", err)
			}

			deleteURL := strings.Split(deactivateURL, "?action=deactivate")[0]
			request, err := http.NewRequest("DELETE", deleteURL, bytes.NewReader([]byte{}))
			if err != nil {
				return http.StatusInternalServerError, fmt.Errorf("Error %v deactivating host %s", err, lastClone["id"].(string))
			}

			request.Header.Set("Content-Type", "application/json")
			_, err = httpClient.Do(request)
			if err != nil {
				return http.StatusInternalServerError, fmt.Errorf("Error %v deleting host %s", err, lastClone["id"].(string))
			}
		}
	}

	return http.StatusOK, nil
}

func (s *ScaleHostDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	if scaleConfig, ok := conf.(model.ScaleHost); ok {
		webhook.ScaleHostConfig = scaleConfig
		webhook.ScaleHostConfig.Type = webhook.Driver
		return nil
	} else if configMap, ok := conf.(map[string]interface{}); ok {
		config := model.ScaleHost{}
		err := mapstructure.Decode(configMap, &config)
		if err != nil {
			return err
		}
		webhook.ScaleHostConfig = config
		webhook.ScaleHostConfig.Type = webhook.Driver
		return nil
	}
	return fmt.Errorf("Can't convert config %v", conf)
}

func (s *ScaleHostDriver) GetDriverConfigResource() interface{} {
	return model.ScaleHost{}
}

func (s *ScaleHostDriver) CustomizeSchema(schema *v1client.Schema) *v1client.Schema {
	return schema
}

func getHosts(hostURL string) (map[string]interface{}, error) {
	hostsResp := make(map[string]interface{})
	resp, err := http.Get(hostURL)
	if err != nil {
		return nil, err
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(respBytes, &hostsResp)
	if err != nil {
		return nil, err
	}

	return hostsResp, nil
}

func createHost(host map[string]interface{}, hostCreateURL string, httpClient *http.Client) (int, error) {
	hostJSON, err := json.Marshal(host)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Error in JSON marshal of host : %v", err)
	}

	request, err := http.NewRequest("POST", hostCreateURL, bytes.NewBuffer(hostJSON))
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Error creating request : %v", err)
	}

	request.Header.Set("Content-Type", "application/json")
	_, err = httpClient.Do(request)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Error creating host : %v", err)
	}

	return http.StatusOK, nil
}

func leftPad(str, pad string, length int) string {
	for {
		if len(str) == length {
			return str[0:length]
		}
		str = pad + str
	}
}
