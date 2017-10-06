package drivers

import (
	"github.com/rancher/go-rancher/v3"
	"github.com/rancher/webhook-service/model"
)

//Drivers map
var Drivers map[string]WebhookDriver

//WebhookDriver interface for all drivers
type WebhookDriver interface {
	ValidatePayload(config interface{}, apiClient *client.RancherClient) (int, error)
	Execute(config interface{}, apiClient *client.RancherClient, requestBody interface{}) (int, error)
	GetDriverConfigResource() interface{}
	ConvertToConfigAndSetOnWebhook(conf interface{}, webhook *model.Webhook) error
	CustomizeSchema(schema *client.Schema) *client.Schema
}

//RegisterDrivers creates object of type driver for every request
func RegisterDrivers() {
	Drivers = map[string]WebhookDriver{}
	Drivers["scaleService"] = &ScaleServiceDriver{}
	Drivers["serviceUpgrade"] = &ServiceUpgradeDriver{}
	Drivers["scaleHost"] = &ScaleHostDriver{}
}

//GetDriver looks up the driver
func GetDriver(key string) WebhookDriver {
	return Drivers[key]
}
