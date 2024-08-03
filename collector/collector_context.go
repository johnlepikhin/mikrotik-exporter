package collector

import (
	"mikrotik-exporter/config"

	routeros "github.com/go-routeros/routeros"
	"github.com/prometheus/client_golang/prometheus"
)

type collectorContext struct {
	ch     chan<- prometheus.Metric
	device *config.Device
	client *routeros.Client
}
