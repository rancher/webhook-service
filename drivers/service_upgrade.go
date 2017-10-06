package drivers

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/v3"
	"github.com/rancher/webhook-service/model"
)

var regTag = regexp.MustCompile(`^[\w]+[\w.-]*`)

type ServiceUpgradeDriver struct {
}

func (s *ServiceUpgradeDriver) ValidatePayload(conf interface{}, apiClient *client.RancherClient) (int, error) {
	config, ok := conf.(model.ServiceUpgrade)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("Can't process config")
	}

	if config.ServiceSelector == nil {
		return http.StatusBadRequest, fmt.Errorf("Service selectors not provided")
	}

	if config.Tag == "" {
		return http.StatusBadRequest, fmt.Errorf("Tag not provided")
	}

	err := IsValidTag(config.Tag)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if config.BatchSize <= 0 {
		return http.StatusBadRequest, fmt.Errorf("Batch size for upgrade not provided/invalid")
	}

	if config.IntervalMillis <= 0 {
		return http.StatusBadRequest, fmt.Errorf("Batch interval for upgrade not provided/invalid")
	}

	return http.StatusOK, nil
}

func (s *ServiceUpgradeDriver) Execute(conf interface{}, apiClient *client.RancherClient, requestPayload interface{}) (int, error) {
	fmt.Println("executed")
	requestBody := make(map[string]interface{})
	config := &model.ServiceUpgrade{}
	err := mapstructure.Decode(conf, config)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "Couldn't unmarshal config")
	}

	requestedTag := config.Tag
	if requestPayload == nil {
		return http.StatusBadRequest, fmt.Errorf("No Payload recevied from webhook")
	}

	requestBody, ok := requestPayload.(map[string]interface{})
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("Body should be of type map[string]interface{}")
	}

	pushedData, ok := requestBody["push_data"]
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("Incomplete webhook response provided")
	}

	pushedTag, ok := pushedData.(map[string]interface{})["tag"].(string)
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("Webhook response contains no tag")
	}

	repository, ok := requestBody["repository"]
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("Response provided without repository information")
	}

	pushedImage := ""
	switch config.PayloadFormat {
	case "alicloud":
		alicloudFullName, fullnameOk := repository.(map[string]interface{})["repo_full_name"].(string)
		alicloudRegion, regionOk := repository.(map[string]interface{})["region"].(string)
		if fullnameOk && regionOk {
			imageName := "registry." + alicloudRegion + ".aliyuncs.com/" + alicloudFullName
			pushedImage = imageName + ":" + pushedTag
		} else {
			return http.StatusBadRequest, fmt.Errorf("Alicloud Docker Hub response provided without image name")
		}
	default:
		imageName, ok := repository.(map[string]interface{})["repo_name"].(string)
		if !ok {
			return http.StatusBadRequest, fmt.Errorf("Response provided without image name")
		}
		pushedImage = imageName + ":" + pushedTag
	}

	if requestedTag != pushedTag {
		return http.StatusOK, nil
	}

	log.Infof("Image %s pushed in Docker Hub, upgrading services with serviceSelector %v", pushedImage, config.ServiceSelector)

	go upgradeServices(apiClient, config, pushedImage)

	return http.StatusOK, nil
}

func upgradeServices(apiClient *client.RancherClient, config *model.ServiceUpgrade, pushedImage string) {
	var key, value string
	var secondaryPresent, primaryPresent bool
	serviceSelector := make(map[string]string)

	for key, value = range config.ServiceSelector {
		serviceSelector[key] = value
	}
	batchSize := config.BatchSize
	intervalMillis := config.IntervalMillis
	startFirst := config.StartFirst
	services, err := apiClient.Service.List(&client.ListOpts{})
	if err != nil {
		log.Errorf("Error %v in listing services", err)
		return
	}

	for _, service := range services.Data {
		secondaryPresent = false
		primaryPresent = false
		primaryLabels := service.LaunchConfig.Labels
		secConfigs := []client.SecondaryLaunchConfig{}
		for _, secLaunchConfig := range service.SecondaryLaunchConfigs {
			labels := secLaunchConfig.Labels
			for k, v := range labels {
				if !strings.EqualFold(k, key) {
					continue
				}
				if !strings.EqualFold(v, value) {
					continue
				}

				secLaunchConfig.ImageUuid = "docker:" + pushedImage
				secLaunchConfig.Labels["io.rancher.container.pull_image"] = "always"
				secConfigs = append(secConfigs, secLaunchConfig)
				secondaryPresent = true
			}
		}

		newLaunchConfig := service.LaunchConfig
		for k, v := range primaryLabels {
			if strings.EqualFold(k, key) {
				if strings.EqualFold(v, value) {
					primaryPresent = true
					newLaunchConfig.ImageUuid = "docker:" + pushedImage
					newLaunchConfig.Labels["io.rancher.container.pull_image"] = "always"
				}
			}
		}

		if !primaryPresent && !secondaryPresent {
			continue
		}

		go func(service client.Service, apiClient *client.RancherClient, newLaunchConfig *client.LaunchConfig,
			secConfigs []client.SecondaryLaunchConfig, primaryPresent bool, secondaryPresent bool) {
			upgStrategy := &client.InServiceUpgradeStrategy{
				BatchSize:      batchSize,
				IntervalMillis: intervalMillis * 1000,
				StartFirst:     startFirst,
			}
			if primaryPresent && secondaryPresent {
				upgStrategy.LaunchConfig = newLaunchConfig
				upgStrategy.SecondaryLaunchConfigs = secConfigs
			} else if primaryPresent && !secondaryPresent {
				upgStrategy.LaunchConfig = newLaunchConfig
			} else if !primaryPresent && secondaryPresent {
				upgStrategy.SecondaryLaunchConfigs = secConfigs
			}

			upgradedService, err := apiClient.Service.ActionUpgrade(&service, &client.ServiceUpgrade{
				InServiceStrategy: upgStrategy,
			})
			if err != nil {
				log.Errorf("Error %v in upgrading service %s", err, service.Id)
				return
			}

			if err := wait(apiClient, upgradedService); err != nil {
				log.Errorln(err)
				return
			}

			if upgradedService.State != "upgraded" {
				return
			}
		}(service, apiClient, newLaunchConfig, secConfigs, primaryPresent, secondaryPresent)
	}
}

func (s *ServiceUpgradeDriver) ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error {
	if upgradeConfig, ok := conf.(model.ServiceUpgrade); ok {
		webhook.ServiceUpgradeConfig = upgradeConfig
		webhook.ServiceUpgradeConfig.Type = webhook.Driver
		return nil
	} else if configMap, ok := conf.(map[string]interface{}); ok {
		config := model.ServiceUpgrade{}
		err := mapstructure.Decode(configMap, &config)
		if err != nil {
			return err
		}
		webhook.ServiceUpgradeConfig = config
		webhook.ServiceUpgradeConfig.Type = webhook.Driver
		return nil
	}
	return fmt.Errorf("Can't convert config %v", conf)
}

func (s *ServiceUpgradeDriver) GetDriverConfigResource() interface{} {
	return model.ServiceUpgrade{}
}

func (s *ServiceUpgradeDriver) CustomizeSchema(schema *client.Schema) *client.Schema {
	options := []string{"dockerhub", "alicloud"}
	minValue := int64(1)

	payloadFormat := schema.ResourceFields["payloadFormat"]
	payloadFormat.Type = "enum"
	payloadFormat.Options = options
	payloadFormat.Default = options[0]
	schema.ResourceFields["payloadFormat"] = payloadFormat

	batchSize := schema.ResourceFields["batchSize"]
	batchSize.Default = 1
	batchSize.Min = &minValue
	schema.ResourceFields["batchSize"] = batchSize

	intervalMillis := schema.ResourceFields["intervalMillis"]
	intervalMillis.Default = 2
	intervalMillis.Min = &minValue
	schema.ResourceFields["intervalMillis"] = intervalMillis

	startFirst := schema.ResourceFields["startFirst"]
	startFirst.Default = false
	schema.ResourceFields["startFirst"] = startFirst

	return schema
}

func wait(apiClient *client.RancherClient, service *client.Service) error {
	for i := 0; i < 36; i++ {
		if err := apiClient.Reload(&service.Resource, service); err != nil {
			return err
		}
		if service.Transitioning != "yes" {
			break
		}
		time.Sleep(5 * time.Second)
	}

	switch service.Transitioning {
	case "yes":
		return fmt.Errorf("Timeout waiting for %s to finish", service.Id)
	case "no":
		return nil
	default:
		return fmt.Errorf("Waiting for %s failed: %s", service.Id, service.TransitioningMessage)
	}
}

// IsValidTag checks if tag valid as per Docker tag convention
func IsValidTag(tag string) error {
	match := regTag.FindAllString(tag, -1)
	if len(match) == 0 || len(match[0]) > 128 || (len(match[0]) != len(tag)) {
		return fmt.Errorf("Invalid tag %s, tag length must be < 128, must contain [a-zA-Z0-9.-_] characters only, cannot start with '.' or '-'", tag)
	}
	return nil
}
