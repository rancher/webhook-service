package model

//ScaleService driver
type ScaleService struct {
	ServiceID   string `json:"serviceId,omitempty" mapstructure:"serviceId"`
	ScaleChange int64  `json:"amount,omitempty" mapstructure:"amount"`
	ScaleAction string `json:"action,omitempty" mapstructure:"action"`
	Min         int64  `json:"min,omitempty" mapstructure:"min"`
	Max         int64  `json:"max,omitempty" mapstructure:"max"`
}
