package metrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/kube/client"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	"github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
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
		"user",
		"vendor-portal",
		"warpdrive",
		"zadig-portal",
		"time-nlp",
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

	CPUPercentage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_percentage",
			Help: "CPU usage percentage",
		},
		[]string{"service", "pod"},
	)

	MemoryPercentage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_percentage",
			Help: "Memory usage percentage",
		},
		[]string{"service", "pod"},
	)

	Healthy = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "healthy",
			Help: "service healthy status",
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

func SetCPUUsagePercentage(serviceName, podName string, usageValue, limitValue int64) {
	CPUPercentage.WithLabelValues(serviceName, podName).Set(float64(usageValue) / float64(limitValue))
}

func SetMemoryUsagePercentage(serviceName, podName string, usageValue, limitValue int64) {
	MemoryPercentage.WithLabelValues(serviceName, podName).Set(float64(usageValue) / float64(limitValue))
}

func SetHealthyStatus(serviceName, podName string, ready bool) {
	if ready {
		Healthy.WithLabelValues(serviceName, podName).Set(1.0)
	} else {
		Healthy.WithLabelValues(serviceName, podName).Set(0.0)
	}
}

func UpdatePodMetrics() error {
	CPU.Reset()
	Memory.Reset()
	CPUPercentage.Reset()
	MemoryPercentage.Reset()
	Healthy.Reset()

	metricsClient, err := client.GetKubeMetricsClient(config.HubServerAddress(), setting.LocalClusterID)
	if err != nil {
		fmt.Printf("failed to get metrics client, err: %v\n", err)
		return err
	}

	podMetriceList, err := metricsClient.PodMetricses(config.Namespace()).List(context.TODO(), v1.ListOptions{})
	if err != nil {
		fmt.Printf("failed to get pod metrics, err: %v\n", err)
		return err
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), setting.LocalClusterID)
	if err != nil {
		return err
	}

	pods, err := getter.ListPods(config.Namespace(), labels.Everything(), kubeClient)
	if err != nil {
		return err
	}
	podMap := make(map[string]*corev1.Pod)
	for _, pod := range pods {
		podMap[pod.Name] = pod
	}
	podMetricsMap := make(map[string]*v1beta1.PodMetrics)
	for _, podMetrics := range podMetriceList.Items {
		tmpPodMetrics := podMetrics
		podMetricsMap[podMetrics.Name] = &tmpPodMetrics
	}

	for _, pod := range pods {
		for _, service := range serviceList {
			if strings.Contains(pod.Name, service) {
				podMetric := podMetricsMap[pod.Name]
				updateResourceMetrics(service, podMetric, pod)
				break
			}
		}
	}

	return nil
}

func updateResourceMetrics(serviceName string, podMetrics *v1beta1.PodMetrics, pod *corev1.Pod) {
	containterMetricsMap := make(map[string]*v1beta1.ContainerMetrics)
	if podMetrics != nil {
		for _, c := range podMetrics.Containers {
			containterMetricsMap[c.Name] = &c
		}
	}

	for _, containter := range pod.Spec.Containers {
		if containter.Name == serviceName {
			containterMetrics := containterMetricsMap[serviceName]
			if containterMetrics == nil {
				SetHealthyStatus(serviceName, pod.Name, false)
				break
			}

			SetHealthyStatus(serviceName, pod.Name, wrapper.Pod(pod).Resource().Ready)

			SetCPUUsage(serviceName, pod.Name, containterMetrics.Usage.Cpu().MilliValue())
			SetMemoryUsage(serviceName, pod.Name, containterMetrics.Usage.Memory().Value())

			SetCPUUsagePercentage(serviceName, podMetrics.Name, containterMetrics.Usage.Cpu().MilliValue(), containter.Resources.Limits.Cpu().MilliValue())
			SetMemoryUsagePercentage(serviceName, podMetrics.Name, containterMetrics.Usage.Memory().Value(), containter.Resources.Limits.Memory().Value())

			break
		}
	}
}
