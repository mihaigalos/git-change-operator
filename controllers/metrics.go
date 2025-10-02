package controllers

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// REST API request metrics
	restAPIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gitchange_rest_api_requests_total",
			Help: "Total number of REST API requests made",
		},
		[]string{"controller", "url", "method", "status_code"},
	)

	restAPIRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gitchange_rest_api_request_duration_seconds",
			Help:    "Duration of REST API requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"controller", "url", "method"},
	)

	restAPIConditionChecks = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gitchange_rest_api_condition_checks_total",
			Help: "Total number of REST API condition checks",
		},
		[]string{"controller", "condition_result"},
	)

	restAPIJSONParsingErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gitchange_rest_api_json_parsing_errors_total",
			Help: "Total number of JSON parsing errors during REST API processing",
		},
		[]string{"controller", "error_type"},
	)

	restAPIResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gitchange_rest_api_response_size_bytes",
			Help:    "Size of REST API responses in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"controller", "url"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(restAPIRequestsTotal)
	metrics.Registry.MustRegister(restAPIRequestDuration)
	metrics.Registry.MustRegister(restAPIConditionChecks)
	metrics.Registry.MustRegister(restAPIJSONParsingErrors)
	metrics.Registry.MustRegister(restAPIResponseSize)
}

// MetricsCollector provides methods to record REST API metrics
type MetricsCollector struct {
	controllerName string
}

// NewMetricsCollector creates a new metrics collector for the specified controller
func NewMetricsCollector(controllerName string) *MetricsCollector {
	return &MetricsCollector{
		controllerName: controllerName,
	}
}

// RecordAPIRequest records metrics for a REST API request
func (m *MetricsCollector) RecordAPIRequest(url, method, statusCode string, duration time.Duration, responseSize int64) {
	restAPIRequestsTotal.WithLabelValues(m.controllerName, url, method, statusCode).Inc()
	restAPIRequestDuration.WithLabelValues(m.controllerName, url, method).Observe(duration.Seconds())
	restAPIResponseSize.WithLabelValues(m.controllerName, url).Observe(float64(responseSize))
}

// RecordConditionCheck records the result of a REST API condition check
func (m *MetricsCollector) RecordConditionCheck(result string) {
	restAPIConditionChecks.WithLabelValues(m.controllerName, result).Inc()
}

// RecordJSONParsingError records a JSON parsing error
func (m *MetricsCollector) RecordJSONParsingError(errorType string) {
	restAPIJSONParsingErrors.WithLabelValues(m.controllerName, errorType).Inc()
}
