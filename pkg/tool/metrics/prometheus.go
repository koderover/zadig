package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	RunningWorkflows = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "running_workflows",
			Help: "Number of currently running workflows",
		},
	)

	PendingWorkflows = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pending_workflows",
			Help: "Number of currently pending workflows",
		},
	)

	RequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "request_total",
			Help: "Number of requests",
		},
		[]string{"method", "handler", "status"},
	)
)

func SetRunningWorkflows(value int64) {
	RunningWorkflows.Set(float64(value))
}

func SetPendingWorkflows(value int64) {
	PendingWorkflows.Set(float64(value))
}

func RegisterRequest(method, handler string, status int) {
	RequestTotal.WithLabelValues(method, handler, fmt.Sprintf("%d", status)).Inc()
}
