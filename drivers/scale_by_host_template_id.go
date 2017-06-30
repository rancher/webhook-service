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

	log "github.com/Sirupsen/logrus"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	v1client "github.com/rancher/go-rancher/client"
	"github.com/rancher/go-rancher/v2"
	rConfig "github.com/rancher/webhook-service/config"
	"github.com/rancher/webhook-service/model"
)

var reg = regexp.MustCompile("[0-9]+$")

type ScaleByHostTemplateIDDriver struct {
}

func (s *ScaleByHostTemplateIDDriver) ValidatePayload(conf interface{}, apiClient *client.RancherClient) (int, error) {
	config, ok := conf.(model.ScaleByHostTemplateID)
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

	if config.HostTemplateID == "" {
		return http.StatusBadRequest, fmt.Errorf("HostTemplateID not provided")
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

	if config.Action == "up" {
		if config.DeleteOption != "" {
			return http.StatusBadRequest, fmt.Errorf("Delete option not to be provided while scaling up")
		}
	}

	if config.Action == "down" {
		if config.DeleteOption != "mostRecent" && config.DeleteOption != "leastRecent" {
			return http.StatusBadRequest, fmt.Errorf("Invalid delete option/Delete option missing %v", config.DeleteOption)
		}
	}

	return http.StatusOK, nil
}

func (s *ScaleByHostTemplateIDDriver) Execute(conf interface{}, apiClient *client.RancherClient, reqBody interface{}) (int, error) {
	var currNameSuffix, baseHostName, currCloneName, suffix string
	var count, index, newHostScale, baseHostIndex int64
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	config := &model.ScaleByHostTemplateID{}
	err := mapstructure.Decode(conf, config)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Couldn't unmarshal config")
	}

	action := config.Action
	amount := config.Amount
	min := config.Min
	max := config.Max
	deleteOption := config.DeleteOption

	cattleConfig := rConfig.GetConfig()
	cattleURL := cattleConfig.CattleURL
	u, err := url.Parse(cattleURL)
	if err != nil {
		panic(err)
	}
	cattleURL = strings.Split(cattleURL, u.Path)[0] + "/v2-beta"

	filters := make(map[string]interface{})
	filters["sort"] = "created"
	filters["order"] = "desc"
	hostCollection, err := apiClient.Host.List(&client.ListOpts{
		Filters: filters,
	})
	if len(hostCollection.Data) == 0 {
		return http.StatusBadRequest, fmt.Errorf("No hosts for scaling found")
	}

	hostScalingGroup := []client.Host{}
	baseHostIndex = -1
	for _, host := range hostCollection.Data {
		if config.HostTemplateID != "" {
			// add hosts with specified hostTemplateId
			if host.HostTemplateId == config.HostTemplateID {
				hostScalingGroup = append(hostScalingGroup, host)
				if host.Driver != "" {
					baseHostIndex = int64(len(hostScalingGroup)) - 1
				}
			}
		}
	}

	if baseHostIndex == -1 && action == "up" {
		return http.StatusBadRequest, fmt.Errorf("Cannot use custom hosts for scaling up")
	}

	count = 0

	if action == "up" {
		// Consider the least recently created as base host for cloning
		// Remove domain from host name, scaleHost12.foo.com becomes scaleHost12
		// Remove largest number suffix from end, scaleHost12 becomes scaleHost
		// Name has precedence over hostname. If name is set, empty this field for the clones
		host := hostScalingGroup[baseHostIndex]
		if host.Name != "" {
			baseHostName = host.Name
		} else {
			baseHostName = host.Hostname
		}
		baseHostName = strings.Split(baseHostName, ".")[0]
		baseSuffix := reg.FindString(baseHostName)
		basePrefix := strings.TrimRight(baseHostName, baseSuffix)

		// Use raw call to get host so as to get additional driver config
		getURL := cattleURL + "/projects/" + host.AccountId + "/hosts/" + host.Id
		log.Infof("Getting config for host %s as base host for cloning", host.Id)
		hostRaw, err := getHostsByTemplateID(getURL, httpClient, cattleConfig.CattleAccessKey, cattleConfig.CattleSecretKey)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		hostCreateURL := cattleURL + "/projects/" + host.AccountId + "/hosts"
		newHostScale = amount + int64(len(hostScalingGroup))
		if newHostScale > max {
			return http.StatusBadRequest, fmt.Errorf("Cannot scale above provided max scale value")
		}

		// Get the most recently created host with same prefix as base host, this will have largest suffix
		suffix = ""
		for _, currentHost := range hostScalingGroup {
			if currentHost.Name != "" {
				currCloneName = currentHost.Name
			} else {
				currCloneName = currentHost.Hostname
			}

			if !strings.Contains(currCloneName, basePrefix) {
				continue
			}

			currCloneName = strings.Split(currCloneName, ".")[0]
			suffix = reg.FindString(currCloneName)
			break
		}

		// if suffix exists, increment by 1, else append '2' to next clone
		for count < amount {
			if suffix != "" {
				prevNumber, err := strconv.Atoi(suffix)
				if err != nil {
					return http.StatusInternalServerError, fmt.Errorf("Error converting %s to int in scaleHost driver: %v", suffix, err)
				}
				currNumber := prevNumber + 1
				currNameSuffix = leftPadInTemplateID(strconv.Itoa(currNumber), "0", len(suffix))
			} else {
				currNameSuffix = "2"
			}

			name := basePrefix + currNameSuffix
			hostRaw["name"] = ""
			hostRaw["hostname"] = name
			//if config.HostTemplateID != "" {  // test whether it will be automatically added
			//	hostRaw["hostTemplateId"] = config.HostTemplateID
			//}

			log.Infof("Creating host with hostname: %s", name)
			code, err := createHostByTemplateID(hostRaw, hostCreateURL, httpClient, cattleConfig.CattleAccessKey, cattleConfig.CattleSecretKey)
			if err != nil {
				return code, err
			}

			suffix = currNameSuffix
			count++
		}
	} else if action == "down" {
		newHostScale = int64(len(hostScalingGroup)) - amount
		if newHostScale < min {
			return http.StatusBadRequest, fmt.Errorf("Cannot scale below provided min scale value")
		}

		badHosts := make(map[string]bool)
		deleteCount := int64(0)
		for _, host := range hostScalingGroup {
			state := host.State
			if state == "inactive" || state == "deactivating" || state == "reconnecting" || state == "disconnected" {
				if deleteCount >= amount {
					return http.StatusBadRequest, fmt.Errorf("Cannot scale down exceed amount")
				}
				badHosts[host.Id] = true
				log.Infof("Deleting host %s with priority because of bad state: %s", host.Id, host.State)
				//if deleteCount > amount, should not deleteHost
				code, err := deleteHostByTemplateID(host.Id, apiClient)
				if err != nil {
					return code, err
				}
				deleteCount++
			}
		}

		amount -= deleteCount
		delIndex := count
		if deleteOption == "mostRecent" {
			log.Infof("Deleting most recently created hosts")
			for count < amount {
				host := hostScalingGroup[delIndex]
				if badHosts[host.Id] {
					delIndex++
					continue
				}
				log.Infof("Deleting host %s", host.Id)
				code, err := deleteHostByTemplateID(host.Id, apiClient)
				if err != nil {
					return code, err
				}
				delIndex++
				count++
			}
		} else if deleteOption == "leastRecent" {
			log.Infof("Deleting least recently created hosts")
			for count < amount {
				index = (int64(len(hostScalingGroup)) - delIndex) - 1
				host := hostScalingGroup[index]
				if badHosts[host.Id] {
					delIndex++
					continue
				}
				log.Infof("Deleting host %s", host.Id)
				code, err := deleteHostByTemplateID(host.Id, apiClient)
				if err != nil {
					return code, err
				}
				delIndex++
				count++
			}
		}
	}

	return http.StatusOK, nil
}

func (s *ScaleByHostTemplateIDDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	if scaleConfig, ok := conf.(model.ScaleByHostTemplateID); ok {
		webhook.ScaleByHostTemplateIDConfig = scaleConfig
		webhook.ScaleByHostTemplateIDConfig.Type = webhook.Driver
		return nil
	} else if configMap, ok := conf.(map[string]interface{}); ok {
		config := model.ScaleByHostTemplateID{}
		err := mapstructure.Decode(configMap, &config)
		if err != nil {
			return err
		}
		webhook.ScaleByHostTemplateIDConfig = config
		webhook.ScaleByHostTemplateIDConfig.Type = webhook.Driver
		return nil
	}
	return fmt.Errorf("Can't convert config %v", conf)
}

func (s *ScaleByHostTemplateIDDriver) GetDriverConfigResource() interface{} {
	return model.ScaleByHostTemplateID{}
}

func (s *ScaleByHostTemplateIDDriver) CustomizeSchema(schema *v1client.Schema) *v1client.Schema {
	scaleOptions := []string{"up", "down"}
	deleteOptions := []string{"mostRecent", "leastRecent"}
	minValue := int64(1)

	action := schema.ResourceFields["action"]
	action.Type = "enum"
	action.Options = scaleOptions
	schema.ResourceFields["action"] = action

	min := schema.ResourceFields["min"]
	min.Default = 1
	min.Min = &minValue
	schema.ResourceFields["min"] = min

	max := schema.ResourceFields["max"]
	max.Default = 100
	max.Min = &minValue
	schema.ResourceFields["max"] = max

	deleteOption := schema.ResourceFields["deleteOption"]
	deleteOption.Type = "enum"
	deleteOption.Options = deleteOptions
	schema.ResourceFields["deleteOption"] = deleteOption

	return schema
}

func getHostsByTemplateID(hostURL string, httpClient *http.Client, accessKey string, secretKey string) (map[string]interface{}, error) {
	hostsResp := make(map[string]interface{})
	request, err := http.NewRequest("GET", hostURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating request to get host: %v", err)
	}

	request.SetBasicAuth(accessKey, secretKey)
	resp, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Error %s in http.Get of host", resp.Status)
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

func createHostByTemplateID(host map[string]interface{}, hostCreateURL string, httpClient *http.Client, accessKey string, secretKey string) (int, error) {
	hostJSON, err := json.Marshal(host)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Error in JSON marshal of host: %v", err)
	}

	request, err := http.NewRequest("POST", hostCreateURL, bytes.NewBuffer(hostJSON))
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Error creating request to create host: %v", err)
	}

	request.SetBasicAuth(accessKey, secretKey)
	request.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(request)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Error creating host: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("Error %s in http.Post while creating host", resp.Status)
	}

	return http.StatusOK, nil
}

func deleteHostByTemplateID(hostID string, apiClient *client.RancherClient) (int, error) {
	_, err := apiClient.ExternalHostEvent.Create(&client.ExternalHostEvent{
		EventType:  "host.evacuate",
		HostId:     hostID,
		DeleteHost: true,
	})

	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Error %v in deleting host %s", err, hostID)
	}

	return http.StatusOK, nil
}

func leftPadInTemplateID(str, pad string, length int) string {
	for {
		if len(str) >= length {
			return str[0:len(str)]
		}
		str = pad + str
	}
}
