package drivers

import (
	"github.com/rancher/go-rancher/v2"
)

//Drivers map
var Drivers map[string]WebhookDriver

//WebhookDriver interface for all drivers
type WebhookDriver interface {
	Execute(payload map[string]interface{}, apiClient client.RancherClient) (int, error)
	GetSchema() interface{}
}

//RegisterDrivers creates object of type driver for every request
func RegisterDrivers() {
	Drivers = map[string]WebhookDriver{}
	Drivers["scaleService"] = &ScaleService{}
}

//GetDriver looks up the driver
func GetDriver(key string) WebhookDriver {
	return Drivers[key]
}
