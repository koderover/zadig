package metrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/log"
)

var (
	Metrics *prometheus.Registry

	serviceList = []string{
		"aslan",
		"cron",
		"dex",
		"discovery",
		"dind",
		"gateway-proxy",
		"gateway",
		"gloo",
		"hub-server",
		"kr-minio",
		"kr-mysql",
		"kr-mongodb",
		"nsqlookup",
		"opa",
		"plutus-vendor",
		"resource-server",
		"vendor-portal",
		"warpdrive",
		"zadig-portal",
	}

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

	CPU = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu",
			Help: "CPU usage",
		},
		[]string{"service", "pod"},
	)

	Memory = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory",
			Help: "Memory usage",
		},
		[]string{"service", "pod"},
	)

	ResponseTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_response_time",
			Help:    "The API response time in seconds",
			Buckets: prometheus.LinearBuckets(0.2, 0.2, 10),
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

func RegisterRequest(startTime int64, method, handler string, status int) {
	RequestTotal.WithLabelValues(method, handler, fmt.Sprintf("%d", status)).Inc()
	ResponseTime.WithLabelValues(method, handler, fmt.Sprintf("%d", status)).Observe(float64(time.Now().UnixMilli()-startTime) / 1000)
}

func SetCPUUsage(serviceName, podName string, value int64) {
	// convert to full core
	CPU.WithLabelValues(serviceName, podName).Set(float64(value) / 1000)
}

func SetMemoryUsage(serviceName, podName string, value int64) {
	// convert to MB
	Memory.WithLabelValues(serviceName, podName).Set(float64(value) / 1024 / 1024)
}

func UpdatePodMetrics() error {
	CPU.Reset()
	Memory.Reset()

	metricsClient, err := client.GetKubeMetricsClient(config.HubServerAddress(), setting.LocalClusterID)
	if err != nil {
		log.Errorf("failed to get metrics client, err: %v", err)
		return err
	}

	podMetrices, err := metricsClient.PodMetricses(config.Namespace()).List(context.TODO(), v1.ListOptions{})

	if err != nil {
		log.Errorf("failed to get pod metrics, err: %v", err)
		return err
	}

	for _, podMetric := range podMetrices.Items {
		for _, service := range serviceList {
			if strings.Contains(podMetric.Name, service) {
				updateResourceMetrics(service, podMetric)
				break
			}
		}
	}

	return nil
}

func updateResourceMetrics(serviceName string, metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == serviceName {
			SetCPUUsage(serviceName, metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage(serviceName, metrics.Name, container.Usage.Memory().Value())
			break
		}
	}
}
