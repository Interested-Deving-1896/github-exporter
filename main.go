package main

import (
	conf "github.com/githubexporter/github-exporter/config"
	"github.com/githubexporter/github-exporter/exporter"
	"github.com/githubexporter/github-exporter/http"
	"github.com/google/go-github/v71/github"
	"github.com/infinityworks/go-common/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	log            *logrus.Logger
	applicationCfg conf.Config
	mets           map[string]*prometheus.Desc
)

func init() {
	applicationCfg = conf.Init()
	mets = exporter.AddMetrics()
	log = logger.Start(&applicationCfg)
}

func main() {
	log.Info("Starting Exporter")

	// TODO - support github app/github enterprise
	exp := exporter.Exporter{
		APIMetrics: mets,
		Config:     applicationCfg,
		Client:     github.NewClient(nil),
	}

	http.NewServer(exp).Start()
}
