package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestServiceConflictsSaveWholeDraftButDoNotHideReadFailures(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusForbidden, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			store, mock := mockStore(t)
			e := reportExecution("service_onboarding")
			e.RootID = e.Run.ID
			listReads := 0
			platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Fatalf("重名冲突不能产生业务写入: %s %s", r.Method, r.URL)
				}
				switch r.URL.Path {
				case "/api/project/products/p":
					w.Write([]byte(`{"product_name":"p","product_feature":{"deploy_type":"helm"}}`))
				case "/api/system/helm/test/index":
					w.Write([]byte(`{"entries":{"service1":[{"version":"1"}],"service2":[{"version":"1"}],"service3":[{"version":"1"}]}}`))
				case "/api/project/products/p/services":
					listReads++
					w.WriteHeader(status)
					w.Write([]byte(`{"services":[["service1","service2"]]}`))
				default:
					t.Fatalf("不应通过目录探测服务或查询构建信息: %s", r.URL)
				}
			})
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT resource_scopes FROM ai_run").WithArgs("run", "user").WillReturnRows(sqlmock.NewRows([]string{"scopes"}).AddRow("[]"))
			mock.ExpectExec("UPDATE ai_run SET context=.*resource_scopes=").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			if status == http.StatusOK {
				expectOnboardingDraftSave(mock)
			}
			s := &AgentService{Store: store, Platform: platform}
			ctx := context.WithValue(context.Background(), executionKey{}, e)
			value, err := nativeSkillTool(t, s, "service_onboarding").Call(ctx, []byte(`{"project_key":"p","project_type":"helm","expected_service_count":3,"services":[{"name":"service1","source":"helm_repository","chart_repo":"test","chart_name":"service1","chart_version":"1"},{"name":"service2","source":"helm_repository","chart_repo":"test","chart_name":"service2","chart_version":"1"},{"name":"service3","source":"helm_repository","chart_repo":"test","chart_name":"service3","chart_version":"1"}]}`))
			if status != http.StatusOK {
				var failure *PlatformError
				if !errors.As(err, &failure) || failure.Status != status || e.Outcome == "awaiting_input" || e.OnboardingDraft != nil {
					t.Fatalf("权限与接口错误不能当成重名补问: %+v %v", value, err)
				}
				return
			}
			if err != nil || e.Outcome != "awaiting_input" || e.OnboardingDraft == nil || len(e.OnboardingDraft.Services) != 3 || e.OnboardingDraft.ExpectedServiceCount != 3 || listReads != 1 {
				t.Fatalf("应保存三个服务和版本，一次读取汇总重名，等待处理: %+v %v", value, err)
			}
			missing := value.(map[string]any)["missing_fields"].([]string)
			if len(missing) != 2 || !strings.Contains(missing[0], "service1 已存在") || !strings.Contains(missing[1], "service2 已存在") {
				t.Fatalf("应只补问全部重名服务，不能强制补充构建: %v", missing)
			}
			_, err = s.submitOperation(ctx, e, models.OperationItem{Action: "create_service", ProjectKey: "p", Params: common.JSON(e.OnboardingDraft.Services[0])})
			var conflict *serviceNameConflict
			if !errors.As(err, &conflict) || listReads != 2 {
				t.Fatalf("提交前仍须重读并拒绝重名服务，不能覆盖: %v", err)
			}
		})
	}
}
