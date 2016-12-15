package model

import (
	"github.com/rancher/go-rancher/client"
)

type ServerAPIError struct {
	client.Resource
	Code    int    `json:"statusCode"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type Webhook struct {
	client.Resource
	URL                string       `json:"url"`
	Driver             string       `json:"driver"`
	Name               string       `json:"name"`
	ScaleServiceConfig ScaleService `json:"scaleServiceConfig"`
}

type WebhookCollection struct {
	client.Collection
	Data []Webhook `json:"data,omitempty"`
}
