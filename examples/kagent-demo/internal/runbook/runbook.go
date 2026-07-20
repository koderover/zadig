package runbook

import (
	"fmt"
	"strings"
)

type Args struct {
	Service string `json:"service" jsonschema:"Service name, for example payment-api, order-worker, or gateway"`
	Symptom string `json:"symptom,omitempty" jsonschema:"Observed symptom, for example latency, error-rate, queue-lag, or timeout"`
}

type Result struct {
	Service        string   `json:"service"`
	Owner          string   `json:"owner"`
	Summary        string   `json:"summary"`
	KeyMetrics     []string `json:"key_metrics"`
	CommonCauses   []string `json:"common_causes"`
	Diagnostics    []string `json:"diagnostics"`
	Escalation     []string `json:"escalation"`
	Risk           string   `json:"risk"`
	MatchedSymptom string   `json:"matched_symptom,omitempty"`
}

func Lookup(args Args) (Result, error) {
	service := normalize(args.Service)
	if service == "" {
		return Result{}, fmt.Errorf("service is required")
	}

	entry, ok := runbooks[service]
	if !ok {
		return Result{}, fmt.Errorf("unknown service %q", service)
	}

	result := entry
	result.Service = service
	result.MatchedSymptom = normalize(args.Symptom)
	if result.MatchedSymptom != "" {
		result.Risk = riskFor(result.Service, result.MatchedSymptom, result.Risk)
	}
	return result, nil
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func riskFor(service, symptom, fallback string) string {
	switch {
	case strings.Contains(symptom, "latency"), strings.Contains(symptom, "timeout"):
		if service == "payment-api" || service == "gateway" {
			return "high"
		}
	case strings.Contains(symptom, "queue"), strings.Contains(symptom, "lag"):
		if service == "order-worker" {
			return "high"
		}
	case strings.Contains(symptom, "error"):
		return "medium"
	}
	return fallback
}

var runbooks = map[string]Result{
	"payment-api": {
		Owner:   "payments-oncall",
		Summary: "Handles payment authorization, capture, refund, and callback status queries.",
		KeyMetrics: []string{
			"http_request_duration_seconds{service=\"payment-api\"}",
			"http_requests_total{service=\"payment-api\",status=~\"5..\"}",
			"payment_provider_timeout_total",
		},
		CommonCauses: []string{
			"Payment provider timeout or elevated error rate",
			"Database connection pool saturation",
			"Idempotency key conflicts during retry bursts",
		},
		Diagnostics: []string{
			"kubectl -n prod get deploy payment-api",
			"kubectl -n prod logs deploy/payment-api --since=15m | grep -i 'timeout\\|provider\\|error'",
			"promql: histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{service=\"payment-api\"}[5m])) by (le))",
		},
		Escalation: []string{
			"Page payments-oncall if p95 latency stays above 800ms for 10 minutes",
			"Escalate to provider-relations if provider timeout rate is above 5%",
		},
		Risk: "medium",
	},
	"order-worker": {
		Owner:   "orders-oncall",
		Summary: "Consumes order events and performs asynchronous order state transitions and compensation.",
		KeyMetrics: []string{
			"queue_lag_seconds{consumer=\"order-worker\"}",
			"worker_job_failures_total{service=\"order-worker\"}",
			"worker_retries_total{service=\"order-worker\"}",
		},
		CommonCauses: []string{
			"Downstream inventory or payment dependency is slow",
			"Poison message causing repeated retries",
			"Worker replicas below expected capacity",
		},
		Diagnostics: []string{
			"kubectl -n prod get deploy order-worker",
			"kubectl -n prod logs deploy/order-worker --since=15m | grep -i 'retry\\|failed\\|panic'",
			"promql: max(queue_lag_seconds{consumer=\"order-worker\"})",
		},
		Escalation: []string{
			"Page orders-oncall if queue lag exceeds 300 seconds",
			"Coordinate with inventory-oncall when inventory dependency errors dominate",
		},
		Risk: "medium",
	},
	"gateway": {
		Owner:   "platform-oncall",
		Summary: "Routes external traffic to backend services and enforces authentication, rate limits, and rollout policy.",
		KeyMetrics: []string{
			"gateway_requests_total{status=~\"5..\"}",
			"gateway_upstream_latency_seconds",
			"gateway_rate_limited_total",
		},
		CommonCauses: []string{
			"Bad route or upstream target configuration",
			"Rate limit policy rejecting healthy traffic",
			"Backend service unavailable or slow",
		},
		Diagnostics: []string{
			"kubectl -n edge get deploy gateway",
			"kubectl -n edge logs deploy/gateway --since=15m | grep -i 'upstream\\|route\\|rate'",
			"promql: sum(rate(gateway_requests_total{status=~\"5..\"}[5m])) by (route)",
		},
		Escalation: []string{
			"Page platform-oncall for sustained 5xx above 2%",
			"Escalate to the owning backend service if failures isolate to one route",
		},
		Risk: "medium",
	},
}
