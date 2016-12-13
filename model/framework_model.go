package model

import (
	v1client "github.com/rancher/go-rancher/client"
)

type ServerAPIError struct {
	v1client.Resource
	Code    int    `json:"statusCode"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type Webhook struct {
	v1client.Resource
	URL                string       `json:"url"`
	Driver             string       `json:"driver"`
	Name               string       `json:"name"`
	State              string       `json:"state"`
	ScaleServiceConfig ScaleService `json:"scaleServiceConfig"`
}

type WebhookCollection struct {
	v1client.Collection
	Data []Webhook `json:"data,omitempty"`
}
