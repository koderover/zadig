package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestServicePodsReuseOpenAPIAndRejectWrongOwnership(t *testing.T) {
	for _, production := range []bool{false, true} {
		p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
			path := "/openapi/environments/"
			if production {
				path += "production/"
			}
			if r.URL.Path != path+"dev/service/api/pods" || r.URL.Query().Get("projectKey") != "demo" ||
				r.Header.Get("Authorization") != "Bearer user" {
				t.Fatalf("没有复用已有 Pod 列表或用户身份: %s", r.URL)
			}
			w.Write([]byte(`{"service_name":"other","pods":[]}`))
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
		var pods []servicePod
		for i := 0; i < 51; i++ {
			pods = append(pods, servicePod{Name: fmt.Sprintf("api-%d", i), Ready: true, ContainersReady: true})
		}
		w.Write(common.JSON(servicePods{ServiceName: "api", Pods: pods}))
	})
	args := models.RepairInput{Target: models.ServiceTarget{ProjectKey: "demo", EnvName: "dev", ServiceName: "api"}, PodNames: []string{"old"}}
	ready, err := (&AgentService{Platform: p}).repairReady(context.Background(), &execution{Authorization: "Bearer user"},
		models.OperationItem{Action: "restart_service", Params: common.JSON(args)})
	if err != nil || ready {
		t.Fatalf("不能用截断后的可见 Pod 子集证明全部就绪: ready=%v err=%v", ready, err)
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
			case "/openapi/environments/production/dev/service/api/pods":
				w.Write([]byte(`{"service_name":"api","pods":[{"pod_name":"api-1","containers":[{"name":"api"},{"name":"sidecar"},{"name":"third"},{"name":"fourth"}]}]}`))
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
		value := (&AgentService{Platform: p}).serviceLogs(context.Background(), &execution{Authorization: "Bearer user"},
			models.ServiceTarget{ProjectKey: "demo", EnvName: "dev", ServiceName: "api", Production: true}).(map[string]any)
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
