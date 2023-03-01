package metrics

import (
	"context"
	"fmt"
	"strings"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func init() {
	Metrics = prometheus.NewRegistry()
	Metrics.MustRegister(RunningWorkflows)
	Metrics.MustRegister(PendingWorkflows)
	Metrics.MustRegister(RequestTotal)
	Metrics.MustRegister(CPU)
	Metrics.MustRegister(Memory)
}

var (
	Metrics *prometheus.Registry

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

func SetCPUUsage(serviceName, podName string, value int64) {
	CPU.WithLabelValues(serviceName, podName).Set(float64(value))
}

func SetMemoryUsage(serviceName, podName string, value int64) {
	Memory.WithLabelValues(serviceName, podName).Set(float64(value))
}

func UpdatePodMetrics() error {
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
		if strings.Contains(podMetric.Name, "aslan") {
			updateAslanResourceMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "cron") {
			updateCronMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "dex") {
			updateDexMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "discovery") {
			updateDiscoveryMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "gateway-proxy") {
			updateGatewayProxyMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "gateway") {
			updateGatewayMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "gloo") {
			updateGlooMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "hub-server") {
			updateHubServerMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "kr-minio") {
			updateMinioMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "kr-mysql") {
			updateMysqlMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "kr-mongodb") {
			updateMongodbMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "nsqlookup") {
			updateNsqlookupMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "dind") {
			updateDindMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "opa") {
			updateOpaMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "resource-server") {
			updateResourceServerMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "warpdrive") {
			updateWarpdriveMetrics(podMetric)
		} else if strings.Contains(podMetric.Name, "zadig-portal") {
			updateZadigPortalMetrics(podMetric)
		}
	}

	return nil
}

func updateAslanResourceMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "aslan" {
			SetCPUUsage("aslan", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("aslan", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateCronMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "cron" {
			SetCPUUsage("cron", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("cron", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateDexMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "dex" {
			SetCPUUsage("dex", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("dex", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateDindMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "dind" {
			SetCPUUsage("dind", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("dind", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateDiscoveryMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "discovery" {
			SetCPUUsage("discovery", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("discovery", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateGatewayMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "gateway" {
			SetCPUUsage("gateway", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("gateway", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateGatewayProxyMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "gateway-proxy" {
			SetCPUUsage("gateway-proxy", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("gateway-proxy", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateGlooMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "gloo" {
			SetCPUUsage("gloo", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("gloo", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateHubServerMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "hub-server" {
			SetCPUUsage("hub-server", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("hub-server", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateMinioMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "kr-minio" {
			SetCPUUsage("kr-minio", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("kr-minio", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateMysqlMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "kr-mysql" {
			SetCPUUsage("kr-mysql", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("kr-mysql", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateMongodbMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "kr-mongodb" {
			SetCPUUsage("kr-mongodb", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("kr-mongodb", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateNsqlookupMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "nsqlookup" {
			SetCPUUsage("nsqlookup", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("nsqlookup", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateOpaMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "opa" {
			SetCPUUsage("opa", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("opa", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateResourceServerMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "resource-server" {
			SetCPUUsage("resource-server", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("resource-server", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateWarpdriveMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "warpdrive" {
			SetCPUUsage("warpdrive", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("warpdrive", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}

func updateZadigPortalMetrics(metrics v1beta1.PodMetrics) {
	for _, container := range metrics.Containers {
		if container.Name == "zadig-portal" {
			SetCPUUsage("zadig-portal", metrics.Name, container.Usage.Cpu().MilliValue())
			SetMemoryUsage("zadig-portal", metrics.Name, container.Usage.Memory().Value())
		}
		break
	}
}
