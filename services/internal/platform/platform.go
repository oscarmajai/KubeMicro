// Package platform holds what every KubeMicro service shares: structured
// logging, Prometheus metrics, health probes and graceful shutdown.
package platform

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "HTTP requests handled, by route and status code.",
	}, []string{"service", "route", "code"})

	latency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency, by route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "route"})
)

// Env returns the environment variable or a fallback.
func Env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// Logger returns a JSON logger tagged with the service name.
func Logger(service string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", service)
}

// WriteJSON writes v as JSON with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes {"error": msg}.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

// Instrument wraps a mux so every request is counted and timed by its route
// pattern (e.g. "GET /products/{id}"), which keeps label cardinality bounded.
func Instrument(service string, mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, route := mux.Handler(r)
		if route == "" {
			route = "unmatched"
		}
		rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
		start := time.Now()
		mux.ServeHTTP(rec, r)
		latency.WithLabelValues(service, route).Observe(time.Since(start).Seconds())
		requests.WithLabelValues(service, route, strconv.Itoa(rec.code)).Inc()
	})
}

// Server runs a service with liveness, readiness and metrics endpoints.
type Server struct {
	Name  string
	Addr  string
	Log   *slog.Logger
	ready atomic.Bool
}

// Run registers the platform endpoints on mux, serves until SIGTERM/SIGINT and
// then drains: readiness flips to 503 first so the Service stops routing new
// traffic before in-flight requests are given time to finish.
func (s *Server) Run(mux *http.ServeMux) error {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !s.ready.Load() {
			WriteError(w, http.StatusServiceUnavailable, "shutting down")
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.Handle("GET /metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:              s.Addr,
		Handler:           Instrument(s.Name, mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		s.Log.Info("listening", "addr", s.Addr)
		errc <- srv.ListenAndServe()
	}()
	s.ready.Store(true)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	s.ready.Store(false)
	s.Log.Info("draining")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	s.Log.Info("stopped")
	return nil
}
