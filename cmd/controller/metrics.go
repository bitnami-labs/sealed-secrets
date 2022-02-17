package main

import (
	"net/http"

	"github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	v1 "k8s.io/api/core/v1"
)

// Define Prometheus Exporter namespace (prefix) for all metric names
const metricNamespace string = "sealed_secrets_controller"

const (
	labelNamespace = "namespace"
	labelName      = "name"
	labelCondition = "condition"
)

var conditionStatusToGaugeValue = map[v1.ConditionStatus]float64{
	v1.ConditionFalse:   -1,
	v1.ConditionUnknown: 0,
	v1.ConditionTrue:    1,
}

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
		[]string{"reason", "namespace"},
	)

	conditionInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricNamespace,
		Name:      "condition_info",
		Help:      "Current SealedSecret condition status. Values are -1 (false), 0 (unknown or absent), 1 (true)",
	}, []string{labelNamespace, labelName, labelCondition})

	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "http_requests_total",
			Help:      "A counter for requests to the wrapped handler.",
		},
		[]string{"path", "code", "method"},
	)

	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "http_request_duration_seconds",
			Help:      "A histogram of latencies for requests.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"path", "method"},
	)
)

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(buildInfo)
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
	prometheus.MustRegister(unsealRequestsTotal)
	prometheus.MustRegister(unsealErrorsTotal)
	prometheus.MustRegister(conditionInfo)
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDurationSeconds)
	// Initialise known label values so that counter exists
	unsealErrorsTotal.WithLabelValues("fetch", "")
}

// ObserveCondition sets a `condition_info` Gauge according to a SealedSecret status.
func ObserveCondition(ssecret *v1alpha1.SealedSecret) {
	if ssecret.Status == nil {
		return
	}
	for _, condition := range ssecret.Status.Conditions {
		conditionInfo.With(prometheus.Labels{
			labelNamespace: ssecret.Namespace,
			labelName:      ssecret.Name,
			labelCondition: string(condition.Type),
		}).Set(conditionStatusToGaugeValue[condition.Status])
	}
}

// UnregisterCondition unregisters Gauges associated to a SealedSecret conditions.
func UnregisterCondition(ssecret *v1alpha1.SealedSecret) {
	if ssecret.Status == nil {
		return
	}
	for _, condition := range ssecret.Status.Conditions {
		conditionInfo.MetricVec.DeleteLabelValues(ssecret.Namespace, ssecret.Name, string(condition.Type))
	}
}

// Instrument HTTP handler
func Instrument(path string, h http.Handler) http.Handler {
	return promhttp.InstrumentHandlerDuration(httpRequestDurationSeconds.MustCurryWith(prometheus.Labels{"path": path}),
		promhttp.InstrumentHandlerCounter(httpRequestsTotal.MustCurryWith(prometheus.Labels{"path": path}), h))
}
