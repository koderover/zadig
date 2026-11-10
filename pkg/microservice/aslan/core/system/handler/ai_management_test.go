package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	ginmiddleware "github.com/koderover/zadig/v2/pkg/middleware/gin"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func TestAIManagementPermissionMatrix(t *testing.T) {
	for _, input := range []struct {
		name       string
		ctx        *internalhandler.Context
		read, edit bool
	}{
		{name: "nil context"},
		{name: "missing resources", ctx: &internalhandler.Context{UserID: "user"}},
		{name: "anonymous", ctx: &internalhandler.Context{Resources: &user.AuthorizedResources{IsSystemAdmin: true}}},
		{name: "ordinary user", ctx: &internalhandler.Context{UserID: "user", Resources: &user.AuthorizedResources{}}},
		{name: "missing AI permissions", ctx: &internalhandler.Context{UserID: "user", Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{}}}},
		{name: "project admin", ctx: &internalhandler.Context{UserID: "user", Resources: &user.AuthorizedResources{ProjectAuthInfo: map[string]*user.ProjectActions{"project": {IsProjectAdmin: true}}}}},
		{name: "system admin", ctx: &internalhandler.Context{UserID: "admin", Resources: &user.AuthorizedResources{IsSystemAdmin: true}}, read: true, edit: true},
		{name: "view", ctx: &internalhandler.Context{UserID: "user", Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{AIManagement: &user.AIManagementActions{View: true}}}}, read: true},
		{name: "edit", ctx: &internalhandler.Context{UserID: "user", Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{AIManagement: &user.AIManagementActions{Edit: true}}}}, read: true, edit: true},
	} {
		t.Run(input.name, func(t *testing.T) {
			if hasAIManagementPermission(input.ctx, false) != input.read || hasAIManagementPermission(input.ctx, true) != input.edit {
				t.Fatal("unexpected AI permission decision")
			}
		})
	}
}

func TestAIManagementHandlersRejectUnauthorizedBeforeDatabaseAccess(t *testing.T) {
	original := newAIManagementAuthorization
	t.Cleanup(func() { newAIManagementAuthorization = original })
	handlers := []struct {
		name    string
		handler gin.HandlerFunc
		edit    bool
	}{
		{"model list", ListLLMIntegration, false}, {"model detail", GetLLMIntegration, false},
		{"model create", CreateLLMIntegration, true}, {"model update", UpdateLLMIntegration, true},
		{"model delete", DeleteLLMIntegration, true}, {"model validate", ValidateLLMIntegration, true},
		{"review read", GetSystemAIReviewConfig, false}, {"review update", UpdateSystemAIReviewConfig, true},
		{"release read", GetAIReleaseSpecialistConfig, false}, {"release update", UpdateAIReleaseSpecialistConfig, true},
	}
	for _, entry := range handlers {
		t.Run(entry.name, func(t *testing.T) {
			for _, authErr := range []error{nil, errors.New("authorization unavailable")} {
				newAIManagementAuthorization = func(*gin.Context) (*internalhandler.Context, error) {
					resources := &user.AuthorizedResources{}
					if entry.edit {
						resources.SystemActions = &user.SystemActions{AIManagement: &user.AIManagementActions{View: true}}
					}
					return &internalhandler.Context{UserID: "user", Resources: resources}, authErr
				}
				r := gin.New()
				r.Use(ginmiddleware.Response())
				r.Any("/", entry.handler)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}")))
				if w.Code != http.StatusForbidden {
					t.Fatalf("expected 403, got %d: %s", w.Code, w.Body)
				}
			}
		})
	}
}

func TestAIReleaseSpecialistConfigRejectsInvalidJSON(t *testing.T) {
	original := newAIManagementAuthorization
	t.Cleanup(func() { newAIManagementAuthorization = original })
	newAIManagementAuthorization = func(*gin.Context) (*internalhandler.Context, error) {
		return &internalhandler.Context{UserID: "user", Resources: &user.AuthorizedResources{SystemActions: &user.SystemActions{AIManagement: &user.AIManagementActions{Edit: true}}}}, nil
	}
	r := gin.New()
	r.Use(ginmiddleware.Response())
	r.PUT("/", UpdateAIReleaseSpecialistConfig)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"llm_integration_id":123}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid parameter response: %d %s", w.Code, w.Body)
	}
}
