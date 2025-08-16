package main

import "github.com/prometheus/client_golang/prometheus"

var (
	diskUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_usage_percent",
			Help: "Disk usage percentage",
		},
		[]string{"disk"},
	)
)

func init() {
	prometheus.MustRegister(diskUsage)
}
