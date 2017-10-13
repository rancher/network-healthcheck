package main

import (
	"fmt"
	"os"

	log "github.com/leodotcloud/log"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher-metadata/metadata"
	"github.com/rancher/network-healthcheck/healthcheck"
	"github.com/rancher/network-healthcheck/server"
	"github.com/urfave/cli"
)

var VERSION = "v0.0.0-dev"

func main() {
	app := cli.NewApp()
	app.Name = "network-healthcheck"
	app.Version = VERSION
	app.Usage = "A healthcheck service for Rancher networking"
	app.Action = run
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug, d",
			EnvVar: "RANCHER_DEBUG",
		},
		cli.BoolFlag{
			Name:   "router-http-check",
			EnvVar: "ROUTER_HTTP_CHECK",
		},
		cli.StringFlag{
			Name:   "metadata-address",
			Usage:  "The metadata service address",
			Value:  "169.254.169.250",
			EnvVar: "RANCHER_METADATA_ADDRESS",
		},
		cli.IntFlag{
			Name:  "health-check-port",
			Usage: "Port to listen on for healthchecks",
			Value: 9898,
		},
	}
	app.Run(os.Args)
}

func run(c *cli.Context) error {
	if c.Bool("debug") {
		log.SetLevelString("debug")
	}
	if c.Bool("router-http-check") {
		server.EnableRouterHTTPCheck()
	}

	mdClient, err := metadata.NewClientAndWait(fmt.Sprintf("http://%s/2016-07-29", c.String("metadata-address")))
	s := server.NewServer(mdClient)

	exit := make(chan error)
	go func(exit chan<- error) {
		err := s.RunLoop()
		exit <- errors.Wrap(err, "Main loop exited")
	}(exit)

	go func(exit chan<- error) {
		err := s.RunRetryLoop()
		exit <- errors.Wrap(err, "Retry loop exited")
	}(exit)

	go func(exit chan<- error) {
		err := healthcheck.StartHealthCheck(c.Int("health-check-port"), s, mdClient)
		exit <- errors.Wrapf(err, "Healthcheck provider died.")
	}(exit)

	err = <-exit
	log.Errorf("Exiting network-healthcheck with error: %v", err)
	return err
}
