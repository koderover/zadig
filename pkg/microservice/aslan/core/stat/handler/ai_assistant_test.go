package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/service"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func TestAIAssistantStatsRoutesRejectAnonymous(t *testing.T) {
	log.Init(&log.Config{Level: "error"})
	r := gin.New()
	(&Router{}).Inject(r.Group("/api/stat"))
	expected := map[string]bool{"overview": false, "trends": false, "skills": false, "rankings": false}
	prefix := "/api/stat/v2/ai_assistant/"
	for _, route := range r.Routes() {
		if !strings.HasPrefix(route.Path, prefix) {
			continue
		}
		name := strings.TrimPrefix(route.Path, prefix)
		if _, ok := expected[name]; !ok || route.Method != http.MethodGet {
			t.Fatalf("统计路由必须为四个明确的 GET 接口: %+v", route)
		}
		expected[name] = true
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, route.Path, nil))
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s 必须在查询数据库或许可证之前拒绝匿名请求: %d", name, w.Code)
		}
		var body e.ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Code != 403 || body.Type != "error" || body.Extra == nil {
			t.Fatalf("权限错误必须使用标准响应: %s %v", w.Body, err)
		}
	}
	for kind, found := range expected {
		if !found {
			t.Fatalf("缺少统计路由: %s", kind)
		}
	}
	for _, name := range []string{"invalid", ":kind", "overview/extra"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, prefix+name, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("不得注册通用报告路由: %s %d", name, w.Code)
		}
	}
}

func TestAIAssistantStatsRequiresReportPermission(t *testing.T) {
	for _, input := range []struct {
		name    string
		ctx     *internalhandler.Context
		allowed bool
	}{
		{name: "无上下文"},
		{name: "匿名身份", ctx: &internalhandler.Context{}},
		{name: "服务身份", ctx: &internalhandler.Context{Resources: &user.AuthorizedResources{IsSystemAdmin: true}}},
		{name: "缺少权限", ctx: &internalhandler.Context{UserID: "user"}},
		{name: "普通用户", ctx: &internalhandler.Context{UserID: "user", Resources: &user.AuthorizedResources{}}},
		{name: "项目管理员", ctx: &internalhandler.Context{UserID: "user", Resources: &user.AuthorizedResources{ProjectAuthInfo: map[string]*user.ProjectActions{"p": {IsProjectAdmin: true}}}}},

		{name: "缺少数据视图权限", ctx: &internalhandler.Context{UserID: "user", Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{}}}},
		{name: "仅效能洞察权限", ctx: &internalhandler.Context{UserID: "user", Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{DataCenter: &user.DataCenterActions{ViewInsight: true}}}}},
		{name: "仅审查分析权限", ctx: &internalhandler.Context{UserID: "user", Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{DataCenter: &user.DataCenterActions{ViewAIReviewInsight: true}}}}},
		{name: "查看助手报告", ctx: &internalhandler.Context{UserID: "user", Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{DataCenter: &user.DataCenterActions{ViewAIAssistantReport: true}}}}, allowed: true},
		{name: "系统管理员", ctx: &internalhandler.Context{UserID: "user", Resources: &user.AuthorizedResources{IsSystemAdmin: true}}, allowed: true},
	} {
		t.Run(input.name, func(t *testing.T) {
			if canViewAIAssistantStats(input.ctx) != input.allowed {
				t.Fatal("统计权限要求用户身份及系统管理员或专用报告权限")
			}
		})
	}
}

func TestAIAssistantStatsErrorsDoNotExposeInternalDetails(t *testing.T) {
	log.Init(&log.Config{Level: "error"})
	for _, input := range []struct {
		err    error
		status int
	}{
		{service.ErrAIAssistantStatsRange, http.StatusBadRequest},
		{service.ErrAIAssistantStatsUnavailable, http.StatusBadRequest},
		{errors.New("private database details"), http.StatusBadRequest},
	} {
		r := gin.New()
		r.GET("/report", func(c *gin.Context) { respondAIAssistantStats(c, nil, input.err) })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/report", nil))
		var body e.ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		wantCode := e.ErrGetAIAssistantStats.Code()
		if errors.Is(input.err, service.ErrAIAssistantStatsRange) {
			wantCode = e.ErrInvalidParam.Code()
		}
		if w.Code != input.status || body.Code != wantCode || body.Type != "error" || body.Extra == nil || strings.Contains(w.Body.String(), "private database details") {
			t.Fatalf("错误状态或脱敏失败: %d %s", w.Code, w.Body)
		}
	}
}
