package drivers

import (
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/webhook-service/model"
)

//Drivers map
var Drivers map[string]WebhookDriver

//WebhookDriver interface for all drivers
type WebhookDriver interface {
	ValidatePayload(config interface{}, apiClient client.RancherClient) (int, error)
	Execute(config interface{}, apiClient client.RancherClient) (int, error)
	GetSchema() interface{}
	ConvertToConfigAndSetOnWebhook(configMap map[string]interface{}, webhook *model.Webhook) error
}

//RegisterDrivers creates object of type driver for every request
func RegisterDrivers() {
	Drivers = map[string]WebhookDriver{}
	Drivers["scaleService"] = &ScaleServiceDriver{}
}

//GetDriver looks up the driver
func GetDriver(key string) WebhookDriver {
	return Drivers[key]
}
