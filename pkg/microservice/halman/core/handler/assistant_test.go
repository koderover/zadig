package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	ormrepo "github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/service"
	builtins "github.com/koderover/zadig/v2/pkg/microservice/halman/core/skill"
)

func TestPermissionFailureReturnsForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	respondError(c, fmt.Errorf("没有创建服务权限: %w", &service.PlatformError{Status: http.StatusForbidden}))
	if w.Code != http.StatusForbidden {
		t.Fatalf("权限不足应返回 403，实际 %d", w.Code)
	}
}

func TestRecommendationsKeepProductionAndValidateInput(t *testing.T) {
	for _, query := range []string{
		"production=true",
		"production=invalid",
		"task_id=-1",
		"batch=-1",
	} {
		t.Run(query, func(t *testing.T) {
			writer := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(writer)
			c.Request = httptest.NewRequest("GET", "/?project_key=p&env_name=prod&service_name=api&"+query, nil)
			api := &API{}
			if query == "production=true" {
				db, mock, err := sqlmock.New()
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				store, err := ormrepo.NewStore(db, 50)
				if err != nil {
					t.Fatal(err)
				}
				rows := sqlmock.NewRows([]string{"id", "source", "enabled"})
				for _, item := range builtins.Builtins() {
					rows.AddRow(item.ID, item.Source, item.Enabled)
				}
				mock.ExpectQuery("SELECT .*FROM `ai_skill`").WithArgs("builtin").WillReturnRows(rows)
				defer func() {
					if err := mock.ExpectationsWereMet(); err != nil {
						t.Error(err)
					}
				}()
				api.Agent = &service.AgentService{Store: store}
			}
			api.recommendations(c)
			if query != "production=true" {
				if writer.Code != 400 {
					t.Fatalf("无效页面参数应拒绝: %d", writer.Code)
				}

				return
			}

			var reply struct {
				Items []models.Recommendation `json:"items"`
			}
			if err := json.Unmarshal(writer.Body.Bytes(), &reply); err != nil || len(reply.Items) != 3 || !reply.Items[0].Context.Production {
				t.Fatalf("推荐应保留页面的生产属性候选: %s %v", writer.Body, err)
			}
		})
	}
}

func TestAssistantErrorsUseExistingBusinessErrorContract(t *testing.T) {
	for _, input := range []struct {
		name         string
		err          error
		status, code int
	}{
		{"参数", fmt.Errorf("%w: 无效字段", common.ErrInvalidRequest), 400, 400},
		{"游标", common.ErrInvalidCursor, 400, 400},
		{"登录", e.NewWithDesc(e.ErrUnauthorized, "登录态无效"), 401, 401},
		{"权限", &service.PlatformError{Status: 403}, 403, 403},
		{"不存在", common.ErrNotFound, 404, 404},
		{"许可证", e.ErrLicenseInvalid, 400, 7050},
		{"开关", common.ErrAIDisabled, 400, 7214},
		{"用户忙碌", common.ErrBusy, 400, 7215},
		{"冲突", common.ErrRequestConflict, 400, 7216},
		{"容量", common.ErrCapacity, 400, 7217},
		{"模型", common.ErrModelUnavailable, 400, 7218},
		{"运行配置", e.ErrAIAssistantRuntime, 400, 7219},
		{"平台访问", &service.PlatformError{Status: 502}, 400, 7220},
		{"写入结果未知", &service.PlatformError{Status: 502, Uncertain: true}, 400, 7221},
		{"许可证服务", e.NewWithDesc(e.ErrInternalError, "许可证查询失败"), 500, 500},
		{"内部故障", errors.New("private database connection details"), 400, 7213},
	} {
		t.Run(input.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			respondError(c, input.err)
			var body e.ErrorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != input.status || body.Code != input.code || body.Type != "error" || body.Message == "" || body.Extra == nil || strings.Contains(recorder.Body.String(), "private database connection details") {
				t.Fatalf("不符合标准错误契约: %d %s", recorder.Code, recorder.Body)
			}
		})
	}
}

func TestAssistantErrorResponsesDoNotMutateSharedDefinitions(t *testing.T) {
	before := e.ErrAIAssistantConflict.Desc()
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		respondError(c, common.ErrRequestConflict)
	}
	if e.ErrAIAssistantConflict.Desc() != before {
		t.Fatal("不能修改共享错误定义")
	}
}

func TestAssistantAGUIErrorsKeepBusinessCodes(t *testing.T) {
	for _, input := range []struct {
		data any
		code int
	}{
		{"保存运行结果失败，请查询运行状态", 7213},
		{e.NewWithDesc(e.ErrUnauthorized, "登录态已失效"), 401},
		{errors.New("private model credentials"), 7213},
	} {
		raw, err := json.Marshal(aguiError(input.data))
		if err != nil {
			t.Fatal(err)
		}
		var body aguievents.RunErrorEvent
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		if body.Code == nil || *body.Code != fmt.Sprint(input.code) || body.Type() != "RUN_ERROR" || body.Message == "" || strings.Contains(string(raw), "private model credentials") {
			t.Fatalf("AG-UI 错误格式不一致: %s", raw)
		}
	}
}
