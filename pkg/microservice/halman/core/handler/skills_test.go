package handler

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	ormrepo "github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/service"
	builtins "github.com/koderover/zadig/v2/pkg/microservice/halman/core/skill"
	"github.com/koderover/zadig/v2/pkg/tool/language"
)

func TestSkillManagementResponsesFollowSystemLanguage(t *testing.T) {
	for _, locale := range []string{"zh-cn", "EN-US", "unknown"} {
		for _, method := range []string{"GET", "PUT"} {
			t.Run(method+"/"+locale, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				store, err := ormrepo.NewStore(db, 50)
				if err != nil {
					t.Fatal(err)
				}
				item := builtins.Builtins()[0]
				item.Enabled, item.CustomRules = false, "保留管理员原文"
				mock.ExpectQuery("SELECT .*FROM `ai_skill`").WillReturnRows(sqlmock.NewRows([]string{"id", "source", "name", "category", "description", "content", "enabled", "custom_rules"}).
					AddRow(item.ID, item.Source, item.Name, item.Category, item.Description, item.Content, item.Enabled, item.CustomRules))
				if method == "PUT" {
					// 保存仍只更新管理员配置，不将英文展示数据写回定义。
					mock.ExpectExec("UPDATE `ai_skill` SET `custom_rules`=\\?,`enabled`=\\?,`updated_at`=\\? WHERE id=\\? AND source=\\?").
						WithArgs(item.CustomRules, false, sqlmock.AnyArg(), item.ID, "builtin").WillReturnResult(sqlmock.NewResult(0, 1))
				}
				writer := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(writer)
				c.Request = httptest.NewRequest(method, "/api/halman/skills/"+item.ID, strings.NewReader(`{"enabled":false,"custom_rules":"保留管理员原文"}`))
				c.Request = c.Request.WithContext(language.WithContext(c.Request.Context(), locale))
				c.Params = gin.Params{{Key: "skill_id", Value: item.ID}}
				api := &API{Agent: &service.AgentService{Store: store}}
				var result models.SkillView
				if method == "GET" {
					api.listSkills(c)
					var body struct {
						Items []models.SkillView `json:"items"`
					}
					if err := json.Unmarshal(writer.Body.Bytes(), &body); err != nil || len(body.Items) != 1 {
						t.Fatalf("列表响应错误: %s %v", writer.Body, err)
					}
					result = body.Items[0]
				} else {
					api.updateSkill(c)
					if err := json.Unmarshal(writer.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if result.UpdatedAt.IsZero() {
						t.Fatal("保存后应保留更新后的时间戳")
					}
					item.UpdatedAt = result.UpdatedAt
				}
				if writer.Code != 200 || !reflect.DeepEqual(result, builtins.Display(item, language.Normalize(locale))) {
					t.Fatalf("管理接口应按请求语言返回展示副本: %s", writer.Body)
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
