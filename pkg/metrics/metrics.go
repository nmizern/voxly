package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	TasksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "voxly_tasks_total",
		Help: "Processed tasks by kind and status.",
	}, []string{"kind", "status"})

	TranscriptionSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "voxly_transcription_seconds",
		Help:    "Transcription latency by provider.",
		Buckets: []float64{0.5, 1, 2, 5, 10, 20, 30, 60, 120, 300},
	}, []string{"provider"})
)

// Handler serves Prometheus metrics at /metrics and a liveness probe at
// /healthz.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}
