package model

//ScaleService driver
type ScaleService struct {
	ServiceID   string  `json:"serviceId,omitempty" mapstructure:"serviceId"`
	ScaleChange float64 `json:"amount,omitempty" mapstructure:"amount"`
	ScaleAction string  `json:"action,omitempty" mapstructure:"action"`
	Min         float64 `json:"min,omitempty" mapstructure:"min"`
	Max         float64 `json:"max,omitempty" mapstructure:"max"`
}
