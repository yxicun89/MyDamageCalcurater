// Package httpmetrics provides the minimal Prometheus instrumentation shared by the Echo v5
// services in this module (gateway・pokedex・calc). It exposes GET /metrics in Prometheus text
// format and counts http_requests_total{method,path,status} plus
// http_request_duration_seconds{method,path} using the default client_golang buckets
// (ADR-0406 §1〜3).
//
// balance・speed・judge (each its own Go module) carry an identical copy of this file
// (ADR-0406 §2: no shared module dependency across service lanes). Any change here must be
// mirrored there.
package httpmetrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Path is the route GET /metrics is served on.
const Path = "/metrics"

// Metrics holds one service instance's independent Prometheus registry and collectors.
type Metrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	handler         http.Handler
}

// New creates a Metrics backed by its own prometheus.Registry (never the global default
// registry), so creating several instances in the same process (as tests do) never panics on
// duplicate registration and never mixes counts across instances.
func New() *Metrics {
	registry := prometheus.NewRegistry()

	requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests, labeled by method, route pattern (not the raw path) and final status code.",
	}, []string{"method", "path", "status"})

	requestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds, labeled by method and route pattern (not the raw path).",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	registry.MustRegister(requestsTotal, requestDuration)

	return &Metrics{
		requestsTotal:   requestsTotal,
		requestDuration: requestDuration,
		handler:         promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	}
}

// Middleware records http_requests_total and http_request_duration_seconds for every request
// except GET /metrics itself (ADR-0406 §3: scraping /metrics must not pollute its own metrics).
//
// It must be registered ahead of (outside) any panic-recovery middleware, so that the status it
// records reflects a recovered panic too. When the handler chain returns an error instead of
// writing the response itself, this middleware runs the service's own echo.HTTPErrorHandler
// right here (so it can read the resulting status) and then returns nil, so Echo's core does not
// run it a second time.
func (m *Metrics) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()
			err := next(c)
			if err != nil {
				c.Echo().HTTPErrorHandler(c, err)
				err = nil
			}

			path := c.Path()
			if path == Path {
				return err
			}

			method := methodLabel(c.Request().Method)
			status := strconv.Itoa(statusOf(c))
			m.requestsTotal.WithLabelValues(method, path, status).Inc()
			m.requestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
			return err
		}
	}
}

// methodLabel normalizes m to one of the standard HTTP methods; anything else becomes "OTHER".
// net/http accepts arbitrary tokens as a request method, so without this the method label would
// have unbounded cardinality (the same cardinality concern ADR-0406 §1 already guards for path by
// using the route pattern instead of the raw path).
func methodLabel(m string) string {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodOptions, http.MethodConnect, http.MethodTrace:
		return m
	default:
		return "OTHER"
	}
}

// statusOf reads the status actually written to the response so far, falling back to the
// zero-value echo.Response status (200) when it cannot unwrap (should not happen in practice).
func statusOf(c *echo.Context) int {
	if r, err := echo.UnwrapResponse(c.Response()); err == nil {
		return r.Status
	}
	return http.StatusOK
}

// Handler serves GET /metrics in Prometheus text format.
func (m *Metrics) Handler() echo.HandlerFunc {
	return func(c *echo.Context) error {
		m.handler.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}
