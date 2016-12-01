package service

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/rancher/webhook-service/drivers"
	"github.com/rancher/webhook-service/webhooks"
	"io/ioutil"
	"net/http"
)

type WebhookRequest map[string]interface{}

func ConstructPayload(w http.ResponseWriter, r *http.Request) {
	var webhookRequestData WebhookRequest
	log.Infof("Construct Payload")
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Construct failed with error: %v", err)
		return
	}
	json.Unmarshal(bytes, &webhookRequestData)
	webhookRequestData["projectId"] = r.Header.Get("X-API-ACCOUNT-ID")
	jwt, err := webhooks.ConstructPayload(webhookRequestData)
	if err != nil {
		fmt.Fprintf(w, "%v", err)
	}
	bytes, err = json.Marshal(jwt)
	fmt.Fprintf(w, "%v\n", string(bytes))
}

func Execute(w http.ResponseWriter, r *http.Request) {
	var payload WebhookRequest
	log.Infof("Execute")
	bytes, err := ioutil.ReadAll(r.Body)
	fmt.Printf("Request headers : %v\n", r.Header)
	if err != nil {
		log.Errorf("Construct failed with error: %v", err)
		return
	}
	json.Unmarshal(bytes, &payload)

	_, resp, err := webhooks.Execute(payload)
	if err != nil {
		fmt.Fprintf(w, "%v", err)
	}
	fmt.Fprintf(w, "%v\n", resp)
	fmt.Printf("Response headers : %v\n", w.Header())
}

func DisplaySchemas(w http.ResponseWriter, r *http.Request) {
	schemasAvailable := drivers.GetDriverSchemas()
	fmt.Fprint(w, "Available Schemas : ", schemasAvailable)
}
