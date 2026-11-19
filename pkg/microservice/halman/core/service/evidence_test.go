package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	kuberesource "github.com/koderover/zadig/v2/pkg/shared/kube/resource"
)

func TestServicePodsUseInternalAPIAndRejectWrongOwnership(t *testing.T) {
	for _, production := range []bool{false, true} {
		p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/environment/environments/dev/services/api" || r.URL.Query().Get("projectName") != "demo" ||
				r.URL.Query().Get("production") != fmt.Sprint(production) ||
				r.Header.Get("Authorization") != "Bearer user" {
				t.Fatalf("没有复用内部服务详情或用户身份: %s", r.URL)
			}
			w.Write([]byte(`{"service_name":"other","scales":[]}`))
		})
		_, err := p.servicePods(context.Background(), "Bearer user", models.ServiceTarget{
			ProjectKey: "demo", EnvName: "dev", ServiceName: "api", Production: production,
		})
		if err == nil {
			t.Fatal("不能接受其他服务的 Pod 列表")
		}
	}
}

func TestTruncatedPodListCannotProveRepairComplete(t *testing.T) {
	p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		var pods []*kuberesource.Pod
		for i := 0; i < 51; i++ {
			pods = append(pods, &kuberesource.Pod{Name: fmt.Sprintf("api-%d", i), PodReady: true, ContainersReady: true})
		}
		w.Write(common.JSON(map[string]any{"service_name": "api", "scales": []kuberesource.Workload{{Pods: pods}}}))
	})
	args := models.RepairInput{Target: models.ServiceTarget{ProjectKey: "demo", EnvName: "dev", ServiceName: "api"}, PodNames: []string{"old"}}
	ready, err := (&AgentService{Platform: p}).repairReady(context.Background(), &execution{Authorization: "Bearer user"},
		models.OperationItem{Action: "restart_service", Params: common.JSON(args)}, time.Time{})
	if err != nil || ready {
		t.Fatalf("不能用截断后的可见 Pod 子集证明全部就绪: ready=%v err=%v", ready, err)
	}
}

func TestRestartDeadlineReportsPodFailureAndAllowsRetry(t *testing.T) {
	finished := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)
	started := finished.Add(-operationTimeout)
	args := models.RepairInput{Target: models.ServiceTarget{ProjectKey: "demo", EnvName: "dev", ServiceName: "api", Production: true}, PodNames: []string{"old", "surge"}}
	item := models.OperationItem{Action: "restart_service", Status: "unknown", Params: common.JSON(args)}
	e := &execution{Run: &models.Run{CreatedAt: started, FinishedAt: &finished}, Authorization: "Bearer user"}
	created := started.Add(2 * time.Second)
	marker := started.Format(time.RFC3339)
	pod := &kuberesource.Pod{Name: "new", CreateTime: created.Unix(), Status: "Pending", Containers: []kuberesource.Container{{
		Name: "api", Image: "registry.example/api:missing", Reason: "ImagePullBackOff", Message: "manifest unknown, token=hidden-value",
	}}}
	workload := kuberesource.Workload{Type: setting.Deployment, Replicas: 1, Pods: []*kuberesource.Pod{{Name: "old"}, pod}}
	s := &AgentService{Platform: testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/environment/environments/dev/services/api":
			w.Write(common.JSON(map[string]any{"service_name": "api", "scales": []kuberesource.Workload{workload}}))
		case "/api/environment/kube/yaml":
			fmt.Fprintf(w, "metadata:\n  name: %s\n  creationTimestamp: %q\n  annotations:\n    kubectl.kubernetes.io/restartedAt: %q\n", pod.Name, created.Format(time.RFC3339), marker)
		default:
			t.Fatalf("unexpected route: %s", r.URL)
		}
	})}
	deadline, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	failure, ok := s.awaitRepair(deadline, e, item).(restartRolloutError)
	if !ok || !strings.Contains(string(failure), "ImagePullBackOff") || !strings.Contains(string(failure), "registry.example/api:missing") || strings.Contains(string(failure), "hidden-value") {
		t.Fatalf("expected a verified and redacted restart failure, got %q", failure)
	}
	status, message, err := s.verifyOperationResult(context.Background(), e, item)
	if err != nil || status != "failed" || message != string(failure) {
		t.Fatalf("expected the prior restart to be eligible for retry: %s, %s, %v", status, message, err)
	}

	marker = ""
	status, _, err = s.verifyOperationResult(context.Background(), e, item)
	if err != nil || status != "unknown" {
		t.Fatalf("an unready pod without evidence of restart must not open a retry: %s, %v", status, err)
	}

	pod.PodReady, pod.ContainersReady = true, true
	workload.Pods = []*kuberesource.Pod{pod}
	if ready, err := s.repairReady(context.Background(), e, item, time.Time{}); err != nil || !ready {
		t.Fatalf("one desired ready pod is enough after replacing two old pods from a stalled rollout: ready=%v, %v", ready, err)
	}
}

func TestServiceLogsReuseExistingRoutesAndBoundEvidence(t *testing.T) {
	for _, large := range []bool{false, true} {
		logs, previous, events := 0, 0, 0
		p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer user" {
				t.Fatal("证据读取只允许携带实际用户身份")
			}
			switch r.URL.Path {
			case "/api/environment/kube/pods/api-1/events":
				events++
				if r.URL.Query().Get("production") != "true" || r.URL.Query().Get("envName") != "dev" {
					t.Fatal("Pod 事件应沿用已有的环境参数")
				}
				w.Write([]byte("[" + strings.TrimSuffix(strings.Repeat(`{"message":"warning"},`, 25), ",") + "]"))
			case "/api/logs/pods/api-1":
				logs++
				q := r.URL.Query()
				if q.Has("production") || q.Get("format") != "excerpt" || q.Get("follow") != "false" ||
					q.Get("tailLines") != "200" || q.Get("serviceName") != "api" || q.Get("projectName") != "demo" || q.Get("envName") != "dev" {
					t.Fatalf("容器日志扩展参数错误: %s", r.URL)
				}
				if q.Get("container") == "fourth" {
					t.Fatal("最多读取三个容器")
				}
				if q.Get("previous") == "true" {
					previous++
					if !large {
						w.WriteHeader(http.StatusBadGateway)
						return
					}
				}
				text := "error"
				if large {
					text = strings.Repeat("x", 48<<10)
				}
				w.Write(common.JSON(map[string]any{"text": text, "truncated": true}))
			default:
				t.Fatalf("不应再调用 assistant 专用日志接口: %s", r.URL)
			}
		})
		pods := &servicePods{ServiceName: "api", Pods: []servicePod{{
			Name: "api-1", Containers: []servicePodContainer{{Name: "api"}, {Name: "sidecar"}, {Name: "third"}, {Name: "fourth"}},
		}}}
		value := (&AgentService{Platform: p}).serviceLogs(context.Background(), &execution{Authorization: "Bearer user"},
			models.ServiceTarget{ProjectKey: "demo", EnvName: "dev", ServiceName: "api", Production: true}, pods).(map[string]any)
		if events != 1 || logs != 6 || previous != 3 {
			t.Fatalf("日志次数错误或重复读取同一 Pod 事件: logs=%d previous=%d events=%d", logs, previous, events)
		}
		if len(common.JSON(value)) > 66<<10 {
			t.Fatal("日志和事件证据突破共享预算")
		}
		items := value["pod_events"].(map[string]any)["api-1"].(map[string]any)
		if !items["truncated"].(bool) || len(items["items"].([]any)) != 20 {
			t.Fatal("Pod 事件没有按条数截断")
		}
		if !large {
			entry := value["container_evidence"].(map[string]any)["api-1/api/previous=true"].(map[string]string)
			if entry["unavailable"] == "" {
				t.Fatal("无法读取上次退出日志时必须保留不可用说明")
			}
		}
	}
}
