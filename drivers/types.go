package drivers

//Drivers map
var Drivers map[string]WebhookDriver

type DriverSchema struct {
	Name        string            `json:"name"`
	InputFormat map[string]string `json:"inputFormat"`
}

//RegisterDrivers creates object of type driver for every request
func RegisterDrivers() {
	Drivers = map[string]WebhookDriver{}
	Drivers["serviceScale"] = &ServiceScaler{}
}

//GetDriver looks up the driver
func GetDriver(key string) WebhookDriver {
	return Drivers[key]
}

func GetDriverSchemas() []DriverSchema {
	var DriverSchemasCollection []DriverSchema
	for key := range Drivers {
		driverSchema := DriverSchema{}
		driverSchema.Name = key
		driverSchema.InputFormat = map[string]string{}
		driverSchema.InputFormat["driver"] = "string"
		switch key {
		case "serviceScale":
			driverSchema.InputFormat["serviceId"] = "string"
			driverSchema.InputFormat["scaleChange"] = "float64"
			driverSchema.InputFormat["scaleAction"] = "string"
			driverSchema.InputFormat["projectId"] = "string"
		}
		DriverSchemasCollection = append(DriverSchemasCollection, driverSchema)
	}
	return DriverSchemasCollection
}
