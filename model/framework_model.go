package model

import (
	"github.com/rancher/go-rancher/v3"
)

type ServerAPIError struct {
	client.Resource
	Code    int    `json:"statusCode"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type Webhook struct {
	client.Resource
	URL                  string         `json:"url"`
	Driver               string         `json:"driver"`
	Name                 string         `json:"name"`
	State                string         `json:"state"`
	ScaleServiceConfig   ScaleService   `json:"scaleServiceConfig"`
	ServiceUpgradeConfig ServiceUpgrade `json:"serviceUpgradeConfig"`
	ScaleHostConfig      ScaleHost      `json:"scaleHostConfig"`
}

type WebhookCollection struct {
	client.Collection
	Data []Webhook `json:"data,omitempty"`
}
