// Package metrics exposes gateway operational data in Prometheus
// exposition format, served from the Admin Endpoint rather than the
// public-facing proxy listener.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Recorder holds every metric the gateway reports.
type Recorder struct {
	Requests            *prometheus.CounterVec
	RequestDuration     *prometheus.HistogramVec
	RateLimitRejections prometheus.Counter
	BackendHealthy      *prometheus.GaugeVec
	BreakerState        *prometheus.GaugeVec
}

// NewRecorder builds and registers every metric on reg.
func NewRecorder(reg prometheus.Registerer) *Recorder {
	r := &Recorder{
		Requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "goway_requests_total",
			Help: "Total proxied requests, labeled by backend and status class.",
		}, []string{"backend", "status_class"}),

		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "goway_request_duration_seconds",
			Help:    "Proxied request latency in seconds, labeled by backend.",
			Buckets: prometheus.DefBuckets,
		}, []string{"backend"}),

		RateLimitRejections: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "goway_ratelimit_rejections_total",
			Help: "Total requests rejected by the rate limiter.",
		}),

		BackendHealthy: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "goway_backend_healthy",
			Help: "1 if the backend's active health check last succeeded, else 0.",
		}, []string{"backend"}),

		BreakerState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "goway_circuit_breaker_state",
			Help: "Circuit breaker state per backend: 0=closed, 1=open, 2=half_open.",
		}, []string{"backend"}),
	}

	reg.MustRegister(r.Requests, r.RequestDuration, r.RateLimitRejections, r.BackendHealthy, r.BreakerState)
	return r
}

// ObserveRequest records one proxied request's outcome and latency.
func (r *Recorder) ObserveRequest(backend string, statusCode int, duration time.Duration) {
	r.Requests.WithLabelValues(backend, statusClass(statusCode)).Inc()
	r.RequestDuration.WithLabelValues(backend).Observe(duration.Seconds())
}

func statusClass(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	default:
		return "2xx"
	}
}
