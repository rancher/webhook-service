package webhooks

import (
	"fmt"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/rancher/webhook-service/drivers"
	"net/http"
)

// ConstructPayload constructs jwt url
func ConstructPayload(input map[string]interface{}) (map[string]interface{}, error) {
	if _, ok := input["driver"].(string); !ok {
		return nil, fmt.Errorf("Driver of type string not provided")
	}
	driverID := input["driver"].(string)
	driver := drivers.GetDriver(driverID)
	if driver == nil {
		return nil, fmt.Errorf("Driver %s is not registered", driverID)
	}
	payload, err := driver.ConstructPayload(input)
	if err != nil {
		return nil, fmt.Errorf("Error %v in driver %s for ConstructPayload", err, driver)
	}
	return payload, nil
}

//Execute does a lookup for driver and calls it
func Execute(payload map[string]interface{}) (int, map[string]interface{}, error) {
	jwtSigned := payload["url"]
	token, err := jwt.Parse(jwtSigned.(string), func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return drivers.PublicKey, nil
	})

	if err != nil || !token.Valid {
		return http.StatusBadRequest, nil, fmt.Errorf("Error %v in token Jwt", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if _, ok := claims["driver"].(string); !ok {
			return http.StatusBadRequest, nil, fmt.Errorf("Driver not found after decode")
		}
		driverID := claims["driver"].(string)
		driver := drivers.GetDriver(driverID)
		if driver == nil {
			return http.StatusBadRequest, nil, fmt.Errorf("Driver %s is not registered", driverID)
		}
		responseCode, response, err := driver.Execute(claims)
		if err != nil {
			return responseCode, nil, fmt.Errorf("Error %v in executing driver for %s", err, driverID)
		}
		return responseCode, response, nil
	}

	return http.StatusBadRequest, nil, fmt.Errorf("Error %v in decoding Jwt", err)
}
