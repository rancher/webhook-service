package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/rancher/go-rancher/v2"
	"github.com/rancher/webhook-service/drivers"
)

func (rh *RouteHandler) Execute(w http.ResponseWriter, r *http.Request) (int, error) {
	var requestBody interface{}

	if r.Body != nil {
		bytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return 500, fmt.Errorf("Error reading request body in Execute handler: %v", err)
		}

		if len(bytes) > 0 {
			err = json.Unmarshal(bytes, &requestBody)
			if err != nil {
				return 500, fmt.Errorf("Error unmarshalling request body in Execute handler: %v", err)
			}
		}
	}

	jwtSigned := r.FormValue("token")
	if jwtSigned != "" {
		code, err := rh.ExecuteWithJwt(jwtSigned, requestBody)
		if err != nil {
			return code, err
		}
		return 200, nil
	}

	uuid := r.FormValue("key")
	if uuid == "" {
		return 400, fmt.Errorf("Invalid execute url, should have 'token' or 'key'")
	}

	projectID := r.FormValue("projectId")
	if projectID == "" {
		return 400, fmt.Errorf("Invalid execute url, url must contain projectId")
	}

	code, err := rh.ExecuteWithKey(uuid, projectID, requestBody)
	if err != nil {
		return code, err
	}

	return 200, nil
}

func (rh *RouteHandler) ExecuteWithJwt(jwtSigned string, requestBody interface{}) (int, error) {
	token, err := jwt.Parse(jwtSigned, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return rh.PublicKey, nil
	})

	if err != nil || !token.Valid {
		return 400, fmt.Errorf("Invalid token error: %v", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		driverID, ok := claims["driver"].(string)
		if !ok {
			return 400, fmt.Errorf("Driver not found after decode")
		}

		driver := drivers.GetDriver(driverID)
		if driver == nil {
			return 400, fmt.Errorf("Driver %s is not registered", driverID)
		}

		projectID, ok := claims["projectId"].(string)
		if !ok {
			return 400, fmt.Errorf("ProjectId not provided by server")
		}

		uuid, ok := claims["uuid"].(string)
		if !ok {
			return 400, fmt.Errorf("Uuid not found after decode")
		}

		apiClient, err := rh.ClientFactory.GetClient(projectID)
		if err != nil {
			return 500, err
		}

		code, err := validateWebhook(uuid, apiClient)
		if err != nil {
			return code, err
		}

		responseCode, err := driver.Execute(claims["config"], apiClient, requestBody)
		if err != nil {
			return responseCode, fmt.Errorf("Error %v in executing driver for %s", err, driverID)
		}
	}
	return 200, nil
}

func (rh *RouteHandler) ExecuteWithKey(uuid string, projectID string, requestBody interface{}) (int, error) {
	apiClient, err := rh.ClientFactory.GetClient(projectID)
	if err != nil {
		return 500, err
	}

	filters := make(map[string]interface{})
	filters["key"] = uuid
	goCollection, err := apiClient.GenericObject.List(&client.ListOpts{
		Filters: filters,
	})
	if err != nil {
		return 500, fmt.Errorf("Error %v filtering genericObjects by key", err)
	}

	if len(goCollection.Data) == 0 {
		return 403, fmt.Errorf("Requested webhook has been revoked/does not exist for this account")
	}

	resourceData := goCollection.Data[0].ResourceData
	driverID, ok := resourceData["driver"].(string)
	if !ok {
		return 400, fmt.Errorf("No driver provided")
	}

	driver := drivers.GetDriver(driverID)
	if driver == nil {
		return 400, fmt.Errorf("Driver %s is not registered", driverID)
	}

	driverConfig, ok := resourceData["config"]
	if !ok {
		return 400, fmt.Errorf("Driver config not found")
	}

	responseCode, err := driver.Execute(driverConfig, apiClient, requestBody)
	if err != nil {
		return responseCode, fmt.Errorf("Error %v in executing driver %s", err, driverID)
	}

	return 200, nil
}

func validateWebhook(uuid string, apiClient *client.RancherClient) (int, error) {
	filters := make(map[string]interface{})
	filters["key"] = uuid
	webhookCollection, err := apiClient.GenericObject.List(&client.ListOpts{
		Filters: filters,
	})
	if err != nil {
		return 500, err
	}
	if len(webhookCollection.Data) > 0 {
		return 0, nil
	}
	return 403, fmt.Errorf("Requested webhook has been revoked")
}
