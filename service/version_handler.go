package service

import (
	"net/http"

	"github.com/rancher/go-rancher/api"
	v1client "github.com/rancher/go-rancher/client"
)

func vHandler(rw http.ResponseWriter, r *http.Request) (int, error) {
	projectID, errCode, err := getProjectID(r)
	if err != nil {
		return errCode, err
	}

	apiContext := api.GetApiContext(r)

	versionResource := v1client.Resource{
		Type:  "apiVersion",
		Links: map[string]string{},
	}

	for _, schema := range schemas.Data {
		if contains(schema.CollectionMethods, "GET") && schema.Id != "apiVersion" && schema.Id != "error" {
			url := apiContext.UrlBuilder.Collection(schema.Id) + "?projectId=" + projectID
			versionResource.Links[schema.PluralName] = url
		}
	}

	selfLink := apiContext.UrlBuilder.Current() + "?projectId=" + projectID
	versionResource.Links["self"] = selfLink
	apiContext.Write(&versionResource)
	return 200, nil
}

func VersionHandler(schemas *v1client.Schemas) http.Handler {
	return api.ApiHandler(schemas, HandleError(schemas, vHandler))
}

func contains(list []string, item string) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}
	return false
}
