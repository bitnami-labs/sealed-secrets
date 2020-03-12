package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Define Prometheus Exporter namespace (prefix) for all metric names
const metricNamespace string = "sealed_secrets_controller"

// Define Prometheus metrics to expose
var (
	buildInfo = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   metricNamespace,
			Name:        "build_info",
			Help:        "Build information.",
			ConstLabels: prometheus.Labels{"revision": VERSION},
		},
	)
	// TODO: rename metric, change increment logic, or accept behaviour
	// when a SealedSecret is deleted the unseal() function is called which is
	// not technically an 'unseal request'.
	unsealRequestsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "unseal_requests_total",
			Help:      "Total number of sealed secret unseal requests",
		},
	)
	unsealErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "unseal_errors_total",
			Help:      "Total number of sealed secret unseal errors by reason",
		},
		[]string{"reason"},
	)
)

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(buildInfo)
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
	prometheus.MustRegister(unsealRequestsTotal)
	prometheus.MustRegister(unsealErrorsTotal)

	// Initialise known label values
	for _, val := range []string{"fetch", "status", "unmanaged", "unseal", "update"} {
		unsealErrorsTotal.WithLabelValues(val)
	}

}
