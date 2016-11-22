package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/rancher/webhook-service/service"
	"github.com/urfave/cli"
	"net/http"
	"os"
)

var VERSION = "v0.0.0-dev"

func main() {
	app := cli.NewApp()
	app.Name = "webhook-service"
	app.Version = VERSION
	app.Usage = "You need help!"
	app.Action = StartWebhook
	app.Commands = []cli.Command{}
	app.Run(os.Args)
}

func StartWebhook(c *cli.Context) {
	router := service.NewRouter()
	log.Infof("Webhook service listening on 8085")
	log.Fatal(http.ListenAndServe(":8085", router))
}
