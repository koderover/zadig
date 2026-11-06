package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestDefaultWorkflowVerificationUsesExistingCompleteLists(t *testing.T) {
	admin := `{"is_system_admin":true}`
	envs := `[{"product_name":"demo","env_name":"staging","is_prod":false}]`
	workflows := `{"workflow_list":[{"name":"demo-workflow-staging","projectName":"demo","workflow_type":"common_workflow"},{"name":"demo-ops-workflow","projectName":"demo","workflow_type":"common_workflow"}],"total":2}`
	for _, input := range []struct {
		name, auth, envs, workflows, want string
		calls                             int
	}{
		{"实际环境名称", admin, envs, workflows, "succeeded", 3},
		{"项目查看权限", `{"project_auth_info":{"demo":{"env":{"View":true},"workflow":{"View":true}}}}`, envs, workflows, "succeeded", 3},
		{"只有协作子集", `{"project_auth_info":{"demo":{}}}`, envs, workflows, "unknown", 1},
		{"没有工作流查看权限", `{"project_auth_info":{"demo":{"env":{"View":true}}}}`, envs, workflows, "unknown", 1},
		{"缺少工作流", admin, envs, `{"workflow_list":[],"total":0}`, "unknown", 3},
		{"列表不完整", admin, envs, `{"workflow_list":[],"total":2}`, "unknown", 3},
		{"同名但项目不符", admin, `[]`, `{"workflow_list":[{"name":"demo-ops-workflow","projectName":"other","workflow_type":"common_workflow"}],"total":1}`, "unknown", 3},
		{"同名但类型不符", admin, `[]`, `{"workflow_list":[{"name":"demo-ops-workflow","projectName":"demo","workflow_type":"other"}],"total":1}`, "unknown", 3},
		{"没有环境只有运维工作流", admin, `[]`, `{"workflow_list":[{"name":"demo-ops-workflow","projectName":"demo","workflow_type":"common_workflow"}],"total":1}`, "succeeded", 3},
	} {
		t.Run(input.name, func(t *testing.T) {
			calls := 0
			p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer user" {
					t.Fatal("核实只能以实际用户身份查询，不能创建资源")
				}
				switch r.URL.Path {
				case "/api/v1/authorization/auth-info":
					if r.URL.Query().Get("uid") != "user" {
						t.Fatal("必须核实当前用户权限")
					}
					w.Write([]byte(input.auth))
				case "/api/environment/environments":
					if r.URL.Query().Get("projectName") != "demo" || r.URL.Query().Get("production") != "false" {
						t.Fatal("默认工作流只匹配实际非生产环境")
					}
					w.Write([]byte(input.envs))
				case "/api/workflow/v4":
					if r.URL.Query().Get("project") != "demo" || r.URL.Query().Has("projectName") {
						t.Fatal("必须沿用现有工作流列表的 project 参数")
					}
					w.Write([]byte(input.workflows))
				default:
					t.Fatalf("不应请求专用核实接口或创建接口: %s", r.URL)
				}
			})
			state, _, err := (&AgentService{Platform: p}).verifyOperationResult(context.Background(),
				&execution{User: "user", Authorization: "Bearer user"},
				models.OperationItem{Action: "create_workflow", ProjectKey: "demo", ProjectType: "k8s"})
			if err != nil || state != input.want || calls != input.calls {
				t.Fatalf("核实结果错误: %s, calls=%d, err=%v", state, calls, err)
			}
		})
	}
}
