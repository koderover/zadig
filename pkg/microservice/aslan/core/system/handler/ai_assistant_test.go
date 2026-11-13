package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func TestAssistantConfigRequiresAIManagementPermission(t *testing.T) {
	originalContext := newAIAssistantContext
	originalLicense := checkAIAssistantEnterpriseLicense
	t.Cleanup(func() {
		newAIAssistantContext = originalContext
		checkAIAssistantEnterpriseLicense = originalLicense
	})
	for _, input := range []struct {
		name      string
		uid       string
		resources *user.AuthorizedResources
		err       error
		allowed   bool
	}{
		{name: "普通用户", uid: "user", resources: &user.AuthorizedResources{}},
		{name: "项目管理员", uid: "user", resources: &user.AuthorizedResources{
			ProjectAuthInfo: map[string]*user.ProjectActions{"project": {IsProjectAdmin: true}},
		}},
		{name: "缺少权限信息", uid: "user"},
		{name: "无用户身份的内部调用", resources: &user.AuthorizedResources{IsSystemAdmin: true}},
		{name: "权限查询失败", uid: "admin", resources: &user.AuthorizedResources{IsSystemAdmin: true}, err: errors.New("unavailable")},
		{name: "仅查看", uid: "viewer", resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{AIManagement: &user.AIManagementActions{View: true}}}, allowed: true},
		{name: "仅编辑", uid: "editor", resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{AIManagement: &user.AIManagementActions{Edit: true}}}, allowed: true},
		{name: "系统管理员", uid: "admin", resources: &user.AuthorizedResources{IsSystemAdmin: true}, allowed: true},
	} {
		t.Run(input.name, func(t *testing.T) {
			newAIAssistantContext = func(*gin.Context) (*internalhandler.Context, error) {
				return &internalhandler.Context{UserID: input.uid, Resources: input.resources}, input.err
			}
			licenseCalls := 0
			checkAIAssistantEnterpriseLicense = func() error {
				licenseCalls++
				return errors.New("许可证服务不可用")
			}
			r := gin.New()
			r.GET("/config", GetAIAssistantConfig)
			r.PUT("/config", UpdateAIAssistantConfig)
			for _, method := range []string{http.MethodGet, http.MethodPut} {
				licenseCalls = 0
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(method, "/config", nil))
				wantStatus, wantCalls := http.StatusForbidden, 0
				if input.allowed && !(input.name == "仅查看" && method == http.MethodPut) {
					wantStatus, wantCalls = http.StatusInternalServerError, 1
				}
				if w.Code != wantStatus || licenseCalls != wantCalls {
					t.Fatalf("%s 状态=%d，许可证调用=%d；期望 %d/%d", method, w.Code, licenseCalls, wantStatus, wantCalls)
				}
			}
		})
	}
}

func TestAssistantConfigRejectsAnonymousBeforeDatabaseAccess(t *testing.T) {
	log.Init(&log.Config{Level: "error"})
	r := gin.New()
	r.GET("/config", GetAIAssistantConfig)
	r.PUT("/config", UpdateAIAssistantConfig)
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/config", nil))
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s 配置必须在访问数据库前拒绝匿名请求，实际 %d", method, w.Code)
		}
		var body e.ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Code != 403 || body.Type != "error" {
			t.Fatalf("权限错误必须使用标准响应: %s %v", w.Body, err)
		}
	}
}

func TestAssistantConfigEnterpriseGuard(t *testing.T) {
	original := checkAIAssistantEnterpriseLicense
	t.Cleanup(func() { checkAIAssistantEnterpriseLicense = original })
	for _, input := range []struct {
		name string
		err  error
		want int
	}{
		{"有效许可证", nil, http.StatusNoContent},
		{"无效许可证", e.NewHTTPError(e.ErrLicenseInvalid.Code(), "许可证无效"), http.StatusBadRequest},
		{"许可证查询失败", errors.New("unavailable"), http.StatusInternalServerError},
	} {
		t.Run(input.name, func(t *testing.T) {
			checkAIAssistantEnterpriseLicense = func() error { return input.err }
			r := gin.New()
			r.GET("/config", func(c *gin.Context) {
				if requireAIAssistantEnterprise(c) {
					c.Status(http.StatusNoContent)
				}
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/config", nil))
			if w.Code != input.want {
				t.Fatalf("企业版校验结果不符: %d", w.Code)
			}
			if input.err != nil {
				var body e.ErrorResponse
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				expected := 500
				if input.want == http.StatusBadRequest {
					expected = e.ErrLicenseInvalid.Code()
				}
				if body.Code != expected || body.Type != "error" || body.Extra == nil {
					t.Fatalf("许可证错误必须使用标准响应: %s", w.Body)
				}
			}
		})
	}
}

func TestAssistantConfigInvalidInputUsesStandardErrorCode(t *testing.T) {
	originalContext, originalLicense := newAIAssistantContext, checkAIAssistantEnterpriseLicense
	t.Cleanup(func() { newAIAssistantContext, checkAIAssistantEnterpriseLicense = originalContext, originalLicense })
	newAIAssistantContext = func(*gin.Context) (*internalhandler.Context, error) {
		return &internalhandler.Context{UserID: "admin", Resources: &user.AuthorizedResources{IsSystemAdmin: true}}, nil
	}
	checkAIAssistantEnterpriseLicense = func() error { return nil }
	r := gin.New()
	r.PUT("/config", UpdateAIAssistantConfig)
	for _, input := range []string{"{", `{"enabled":false,"history_retention_days":1}`} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/config", strings.NewReader(input)))
		var body e.ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != 400 || body.Code != 400 || body.Type != "error" || body.Description == "" || body.Extra == nil {
			t.Fatalf("无效配置应在访问数据库前返回标准参数错误: %d %s", w.Code, w.Body)
		}
	}
}
