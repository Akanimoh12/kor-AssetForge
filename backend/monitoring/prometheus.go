package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PrometheusMetrics struct {
	HTTPRequestsTotal   prometheus.CounterVec
	HTTPRequestDuration prometheus.HistogramVec
	HTTPResponseSize    prometheus.HistogramVec
	DBQueryDuration     prometheus.HistogramVec
	DBErrors            prometheus.CounterVec
	CacheHits           prometheus.CounterVec
	CacheMisses         prometheus.CounterVec
	ActiveConnections   prometheus.GaugeVec
}

func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		HTTPRequestsTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		HTTPResponseSize: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "HTTP response size in bytes",
				Buckets: []float64{100, 1000, 10000, 100000, 1000000},
			},
			[]string{"method", "path"},
		),
		DBQueryDuration: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "db_query_duration_seconds",
				Help:    "Database query duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"query_type", "table"},
		),
		DBErrors: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "db_errors_total",
				Help: "Total number of database errors",
			},
			[]string{"query_type", "table"},
		),
		CacheHits: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"cache_type"},
		),
		CacheMisses: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"cache_type"},
		),
		ActiveConnections: *promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "active_connections",
				Help: "Number of active connections",
			},
			[]string{"connection_type"},
		),
	}
}
