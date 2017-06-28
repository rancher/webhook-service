package model

//ScaleService driver
type ScaleService struct {
	ServiceID   string `json:"serviceId,omitempty" mapstructure:"serviceId"`
	ScaleChange int64  `json:"amount,omitempty" mapstructure:"amount"`
	ScaleAction string `json:"action,omitempty" mapstructure:"action"`
	Min         int64  `json:"min,omitempty" mapstructure:"min"`
	Max         int64  `json:"max,omitempty" mapstructure:"max"`
	Type        string `json:"type,omitempty" mapstructure:"type"`
}

//ServiceUpgrade driver
type ServiceUpgrade struct {
	ServiceSelector map[string]string `json:"serviceSelector,omitempty" mapstructure:"serviceSelector"`
	Tag             string            `json:"tag,omitempty" mapstructure:"tag"`
	PayloadFormat   string            `json:"payloadFormat,omitempty" mapstructure:"payloadFormat"`
	BatchSize       int64             `json:"batchSize,omitempty" mapstructure:"batchSize"`
	IntervalMillis  int64             `json:"intervalMillis,omitempty" mapstructure:"intervalMillis"`
	StartFirst      bool              `json:"startFirst,omitempty" mapstructure:"startFirst"`
	Type            string            `json:"type,omitempty" mapstructure:"type"`
}

//ScaleHost driver
type ScaleHost struct {
	HostSelector   map[string]string `json:"hostSelector,omitempty" mapstructure:"hostSelector"`
	HostTemplateID string            `json:"hostTemplateId,omitempty" mapstructure:"hostTemplateId"`
	Amount         int64             `json:"amount,omitempty" mapstructure:"amount"`
	Action         string            `json:"action,omitempty" mapstructure:"action"`
	Min            int64             `json:"min,omitempty" mapstructure:"min"`
	Max            int64             `json:"max,omitempty" mapstructure:"max"`
	DeleteOption   string            `json:"deleteOption,omitempty" mapstructure:"deleteOption"`
	Type           string            `json:"type,omitempty" mapstructure:"type"`
}
