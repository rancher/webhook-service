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

//ScaleHost driver
type ScaleHost struct {
	HostID string `json:"hostId,omitempty" mapstructure:"hostId"`
	Amount int64  `json:"amount,omitempty" mapstructure:"amount"`
	Action string `json:"action,omitempty" mapstructure:"action"`
	Min    int64  `json:"min,omitempty" mapstructure:"min"`
	Max    int64  `json:"max,omitempty" mapstructure:"max"`
	Type   string `json:"type,omitempty" mapstructure:"type"`
}
