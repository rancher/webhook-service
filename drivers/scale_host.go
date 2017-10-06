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
	"github.com/rancher/go-rancher/v3"
	rConfig "github.com/rancher/webhook-service/config"
	"github.com/rancher/webhook-service/model"
)

var re = regexp.MustCompile("[0-9]+$")

type ScaleHostDriver struct {
}

//ValidatePayload function should not check validation of hostSelector field, since it will be deprecated
func (s *ScaleHostDriver) ValidatePayload(conf interface{}, apiClient *client.RancherClient) (int, error) {
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

	if config.HostTemplateID == "" {
		return http.StatusBadRequest, fmt.Errorf("hostTemplateId is not provided")
	}

	hostTemplate, err := apiClient.HostTemplate.ById(config.HostTemplateID)
	if err != nil {
		log.Errorf("Cannot get hostTemplate resource: %v", err)
		return http.StatusBadRequest, fmt.Errorf("Cannot get hostTemplate resource")
	}

	if hostTemplate == nil || hostTemplate.Removed != "" {
		return http.StatusBadRequest, fmt.Errorf("hostTemplate does not exist")
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

func (s *ScaleHostDriver) Execute(conf interface{}, apiClient *client.RancherClient, reqBody interface{}) (int, error) {
	var currNameSuffix, baseHostName, currCloneName, suffix, key, value string
	var count, newHostScale, baseHostIndex int64

	config := &model.ScaleHost{}
	err := mapstructure.Decode(conf, config)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Couldn't unmarshal config")
	}

	action := config.Action
	amount := config.Amount
	max := config.Max

	if config.HostTemplateID != "" { // logic for scale host with hostTemplateId
		hostTemplate, err := apiClient.HostTemplate.ById(config.HostTemplateID)
		if err != nil {
			log.Errorf("Cannot get hostTemplate resource: %v", err)
			return http.StatusBadRequest, fmt.Errorf("Cannot get hostTemplate resource")
		}

		if hostTemplate == nil || hostTemplate.Removed != "" {
			return http.StatusBadRequest, fmt.Errorf("hostTemplate does not exist")
		}

		filters := make(map[string]interface{})
		filters["sort"] = "created"
		filters["order"] = "desc"
		hostCollection, err := apiClient.Host.List(&client.ListOpts{
			Filters: filters,
		})

		hostScalingGroup := []client.Host{}
		baseHostIndex = -1
		for _, host := range hostCollection.Data {
			if host.HostTemplateId == config.HostTemplateID {
				hostScalingGroup = append(hostScalingGroup, host)
				if host.Driver != "" {
					baseHostIndex = int64(len(hostScalingGroup)) - 1
				}
			}
		}

		if baseHostIndex == -1 {
			baseHostName = "scaledhost"
		} else {
			host := hostScalingGroup[baseHostIndex]
			if host.Name != "" {
				baseHostName = host.Name
			} else {
				baseHostName = host.Hostname
			}
			baseHostName = strings.Split(baseHostName, ".")[0]
		}

		count = 0

		if action == "up" {
			baseSuffix := re.FindString(baseHostName)
			basePrefix := strings.TrimRight(baseHostName, baseSuffix)

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
				suffix = re.FindString(currCloneName)
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
					currNameSuffix = leftPad(strconv.Itoa(currNumber), "0", len(suffix))
				} else if baseHostIndex == -1 {
					currNameSuffix = "1" //since there is not host with the specified hostTemplateId exsited before
				} else {
					currNameSuffix = "2"
				}

				hst := client.Host{}
				name := basePrefix + currNameSuffix
				hst.Name = ""
				hst.Hostname = name
				hst.HostTemplateId = hostTemplate.Id
				log.Infof("Creating host with hostname: %s", name)

				_, err := apiClient.Host.Create(&hst)
				if err != nil {
					log.Errorf("Cannot create host: %v", err)
					return http.StatusInternalServerError, fmt.Errorf("Cannot create host")
				}

				suffix = currNameSuffix
				count++
			}
		} else if action == "down" {
			return scaleDown(hostScalingGroup, config, apiClient)
		}
	} else { // logic for scale host with labels
		httpClient := &http.Client{
			Timeout: time.Second * 10,
		}

		hostSelector := make(map[string]string)
		if config.HostSelector != nil {
			for key, value = range config.HostSelector {
				hostSelector[key] = value
			}
		}

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
		hostSelectorPresent := false
		baseHostIndex = -1
		for _, host := range hostCollection.Data {
			labels := host.Labels
			labelFound := false
			for k, v := range labels {
				if !strings.EqualFold(k, key) {
					continue
				}
				if !strings.EqualFold(v, value) {
					continue
				}
				labelFound = true
				break
			}

			if !labelFound {
				continue
			}

			if host.State == "error" {
				continue
			}

			hostSelectorPresent = true
			hostScalingGroup = append(hostScalingGroup, host)

			if host.Driver != "" {
				baseHostIndex = int64(len(hostScalingGroup)) - 1
			}
		}

		if hostSelectorPresent == false {
			return http.StatusBadRequest, fmt.Errorf("No host with label %v exists", hostSelector)
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
			baseSuffix := re.FindString(baseHostName)
			basePrefix := strings.TrimRight(baseHostName, baseSuffix)

			// Use raw call to get host so as to get additional driver config
			//Todo: figure out how to retrieve host.id
			getURL := cattleURL + "/projects/" + "" + "/hosts/" + host.Id
			log.Infof("Getting config for host %s as base host for cloning", host.Id)

			hostRaw, err := getHosts(getURL, httpClient, cattleConfig.CattleAccessKey, cattleConfig.CattleSecretKey)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			//Todo: figure out url
			hostCreateURL := cattleURL + "/projects/" + "" + "/hosts"
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
				suffix = re.FindString(currCloneName)
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
					currNameSuffix = leftPad(strconv.Itoa(currNumber), "0", len(suffix))
				} else {
					currNameSuffix = "2"
				}

				name := basePrefix + currNameSuffix
				hostRaw["name"] = ""
				hostRaw["hostname"] = name

				log.Infof("Creating host with hostname: %s", name)
				code, err := createHost(hostRaw, hostCreateURL, httpClient, cattleConfig.CattleAccessKey, cattleConfig.CattleSecretKey)
				if err != nil {
					log.Errorf("Cannot create host: %v", err)
					return code, fmt.Errorf("Cannot create host")
				}

				suffix = currNameSuffix
				count++
			}
		} else if action == "down" {
			return scaleDown(hostScalingGroup, config, apiClient)
		}
	}

	return http.StatusOK, nil
}

func scaleDown(hostScalingGroup []client.Host, config *model.ScaleHost, apiClient *client.RancherClient) (int, error) {
	amount := config.Amount
	min := config.Min
	deleteOption := config.DeleteOption

	var newHostScale int64
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
			code, err := deleteHost(host.Id, apiClient)
			if err != nil {
				log.Errorf("Cannot delete host: %v", err)
				return code, fmt.Errorf("Cannot delete host")
			}
			deleteCount++
		}
	}

	count := int64(0)
	delIndex := count
	amount -= deleteCount
	if deleteOption == "mostRecent" {
		log.Infof("Deleting most recently created hosts")
		for count < amount {
			host := hostScalingGroup[delIndex]
			if badHosts[host.Id] {
				delIndex++
				continue
			}
			log.Infof("Deleting host %s", host.Id)
			code, err := deleteHost(host.Id, apiClient)
			if err != nil {
				log.Errorf("Cannot delete host: %v", err)
				return code, fmt.Errorf("Cannot delete host")
			}
			delIndex++
			count++
		}
	} else if deleteOption == "leastRecent" {
		log.Infof("Deleting least recently created hosts")
		index := int64(0)
		for count < amount {
			index = (int64(len(hostScalingGroup)) - delIndex) - 1
			host := hostScalingGroup[index]
			if badHosts[host.Id] {
				delIndex++
				continue
			}
			log.Infof("Deleting host %s", host.Id)
			code, err := deleteHost(host.Id, apiClient)
			if err != nil {
				log.Errorf("Cannot delete host: %v", err)
				return code, fmt.Errorf("Cannot delete host")
			}
			delIndex++
			count++
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

func (s *ScaleHostDriver) CustomizeSchema(schema *client.Schema) *client.Schema {
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

func getHosts(hostURL string, httpClient *http.Client, accessKey string, secretKey string) (map[string]interface{}, error) {
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

func createHost(host map[string]interface{}, hostCreateURL string, httpClient *http.Client, accessKey string, secretKey string) (int, error) {
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

func deleteHost(hostID string, apiClient *client.RancherClient) (int, error) {
	host, err := apiClient.Host.ById(hostID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Can't get host from hostID %s", hostID)
	}
	err = apiClient.Host.Delete(host)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Error %v in deleting host %s", err, hostID)
	}

	return http.StatusOK, nil
}

func leftPad(str, pad string, length int) string {
	for {
		if len(str) >= length {
			return str[0:len(str)]
		}
		str = pad + str
	}
}
