package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/service"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

type skillAuthTransport func(*http.Request) (*http.Response, error)

func (f skillAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSkillManagementRequiresAdminAndWorksWhenAIDisabled(t *testing.T) {
	log.Init(&log.Config{Level: "error"})
	store, mock := mockStore(t)
	admin, permissionFailure := false, false
	users := &userclient.Client{Client: httpclient.New(httpclient.SetHostURL("http://user/api/v1"))}
	users.SetTransport(skillAuthTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/authorization/auth-info" || r.URL.Query().Get("uid") != "actual-user" {
			t.Fatalf("必须查询已鉴权用户的系统权限: %s", r.URL)
		}
		status, body := http.StatusOK, `{"is_system_admin":false}`
		if admin {
			body = `{"is_system_admin":true}`
		}
		if permissionFailure {
			status, body = http.StatusInternalServerError, "private permission details"
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	}))
	platform := &service.PlatformClient{
		Users: users,
		Auth: fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
			return &authv3.CheckResponse{Status: &rpcstatus.Status{}, HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}, nil
		}},
		CheckEnterpriseLicense: func() error { return nil },
		LoadRuntime: func(context.Context) (*types.AIAssistantRuntime, error) {
			return &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{HistoryRetentionDays: 90}}, nil
		},
	}
	router := NewEngine(&service.AgentService{Store: store, Platform: platform})
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", token("actual-user"))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		path := "/api/halman/skills"
		if method == http.MethodPut {
			path += "/workflow_diagnosis"
		}
		if rec := request(method, path, "{}"); rec.Code != http.StatusForbidden {
			t.Fatalf("普通用户不得管理: %d %s", rec.Code, rec.Body)
		}
	}
	admin = true
	mock.ExpectQuery("SELECT .*FROM `ai_skill`").WithArgs("builtin").WillReturnRows(sqlmock.NewRows([]string{"id", "source", "content", "enabled"}).AddRow("workflow_diagnosis", "builtin", "中文内置正文", true))
	if rec := request(http.MethodGet, "/api/halman/skills", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "中文内置正文") || strings.Contains(rec.Body.String(), "prompt") {
		t.Fatalf("总开关关闭时仍应展示完整定义，且不返回默认提问: %d %s", rec.Code, rec.Body)
	}
	path := "/api/halman/skills/workflow_diagnosis"
	for _, body := range []string{"{}", `{"enabled":true}`, `{"enabled":true,"custom_rules":null}`, `{"enabled":true,"custom_rules":"","content":"覆盖正文"}`, `{"enabled":true,"custom_rules":""} {}`, `{"enabled":true,"custom_rules":"` + strings.Repeat("a", 8001) + `"}`, "{\xff}"} {
		rec := request(http.MethodPut, path, body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("应在访问数据库前拒绝非法写入: %d %s", rec.Code, rec.Body)
		}
		assertAssistantErrorCode(t, rec, e.ErrInvalidParam.Code())
	}
	for _, enabled := range []bool{false, true} {
		mock.ExpectQuery("SELECT .*FROM `ai_skill`").WithArgs("workflow_diagnosis", "builtin", 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "source", "content", "enabled"}).AddRow("workflow_diagnosis", "builtin", "只读正文", true))
		mock.ExpectExec("UPDATE `ai_skill` SET `custom_rules`=.*`enabled`=.*`updated_at`=").WithArgs("", enabled, sqlmock.AnyArg(), "workflow_diagnosis", "builtin").WillReturnResult(sqlmock.NewResult(0, 1))
		body, _ := json.Marshal(map[string]any{"enabled": enabled, "custom_rules": ""})
		rec := request(http.MethodPut, path, string(body))
		var item models.Skill
		if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil || rec.Code != http.StatusOK || item.Enabled != enabled || item.Content != "只读正文" {
			t.Fatalf("停用及恢复默认必须保留正文: %d %s %v", rec.Code, rec.Body, err)
		}
	}
	mock.ExpectQuery("SELECT .*FROM `ai_skill`").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	if rec := request(http.MethodPut, path, `{"enabled":true,"custom_rules":""}`); rec.Code != http.StatusNotFound {
		t.Fatalf("未知 ID 不得被创建: %d %s", rec.Code, rec.Body)
	}
	permissionFailure = true
	if rec := request(http.MethodGet, "/api/halman/skills", ""); rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "private") {
		t.Fatalf("权限查询失败必须关闭访问并脱敏: %d %s", rec.Code, rec.Body)
	}
}

func TestRecommendationEndpointReadsEnabledSkills(t *testing.T) {
	router, mock := aguiTestEngine(t)
	mock.ExpectQuery("SELECT .*FROM `ai_skill`").WithArgs("builtin").WillReturnRows(sqlmock.NewRows([]string{"id", "source", "enabled"}).
		AddRow("workflow_diagnosis", "builtin", false).AddRow("project_onboarding", "builtin", true))
	req := httptest.NewRequest(http.MethodGet, "/api/halman/recommendations", nil)
	req.Header.Set("Authorization", token("user"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var response struct {
		Items []models.Recommendation `json:"items"`
		Next  int                     `json:"next_batch"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || rec.Code != http.StatusOK || len(response.Items) != 1 || response.Items[0].SkillID != "project_onboarding" || response.Next != 0 {
		t.Fatalf("只应返回启用项，不重复补齐: %d %s %v", rec.Code, rec.Body, err)
	}
}
