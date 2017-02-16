package service

import (
	"fmt"
	"time"

	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/webhook-service/config"
)

type RancherClientFactory interface {
	GetClient(projectID string) (*client.RancherClient, error)
}

type ClientFactory struct{}

func (f *ClientFactory) GetClient(projectID string) (*client.RancherClient, error) {
	config := config.GetConfig()
	url := fmt.Sprintf("%s/projects/%s/schemas", config.CattleURL, projectID)
	apiClient, err := client.NewRancherClient(&client.ClientOpts{
		Timeout:   time.Second * 30,
		Url:       url,
		AccessKey: config.CattleAccessKey,
		SecretKey: config.CattleSecretKey,
	})
	if err != nil {
		return &client.RancherClient{}, fmt.Errorf("Error in creating API client")
	}
	return apiClient, nil
}
