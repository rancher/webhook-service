package model

//ScaleService driver
type ScaleService struct {
	ServiceID   string  `json:"serviceId,omitempty" mapstructure:"serviceId"`
	ScaleChange float64 `json:"amount,omitempty" mapstructure:"amount"`
	ScaleAction string  `json:"action,omitempty" mapstructure:"action"`
}
