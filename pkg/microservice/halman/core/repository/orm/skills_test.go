package orm

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	builtins "github.com/koderover/zadig/v2/pkg/microservice/halman/core/skill"
)

func skillRows(item models.Skill) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "source", "name", "category", "description", "content", "enabled", "custom_rules"}).
		AddRow(item.ID, item.Source, item.Name, item.Category, item.Description, item.Content, item.Enabled, item.CustomRules)
}

func TestBuiltinSkillInitializationAndUpgradePreserveOverrides(t *testing.T) {
	s, mock := mockStore(t)
	definitions := builtins.Builtins()
	if len(definitions) != 6 {
		t.Fatal("应包含六个内置 Skill")
	}
	for _, upgrade := range []bool{false, true} {
		mock.ExpectBegin()
		for i, item := range definitions {
			for _, section := range []string{"适用场景", "所需信息", "处理步骤", "补问方式", "输出要求", "完成条件"} {
				if !strings.Contains(item.Content, "## "+section) {
					t.Fatalf("%s 缺少中文章节 %s", item.ID, section)
				}
			}
			query := mock.ExpectQuery("SELECT .*FROM `ai_skill` WHERE id=").WithArgs(item.ID, 1)
			if !upgrade {
				query.WillReturnRows(sqlmock.NewRows([]string{"id"}))
				mock.ExpectExec("INSERT INTO `ai_skill`").WithArgs(item.ID, "builtin", item.Name, item.Category, item.Description, item.Content, true, "", sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
				continue
			}
			existing := item
			existing.Enabled, existing.CustomRules = false, "项目标识使用团队前缀"
			if i == 0 {
				existing.Content = "旧版正文"
			}
			query.WillReturnRows(skillRows(existing))
			if i == 0 {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE `ai_skill` SET `category`=?,`content`=?,`description`=?,`name`=?,`updated_at`=? WHERE id=? AND source=?")).
					WithArgs(item.Category, item.Content, item.Description, item.Name, sqlmock.AnyArg(), item.ID, "builtin").WillReturnResult(sqlmock.NewResult(0, 1))
			}
		}
		mock.ExpectCommit()
		if err := s.syncBuiltinSkills(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBuiltinSynchronizationDoesNotOverwriteOtherSources(t *testing.T) {
	s, mock := mockStore(t)
	item := builtins.Builtins()[0]
	item.Source = "custom"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .*FROM `ai_skill`").WillReturnRows(skillRows(item))
	mock.ExpectRollback()
	if err := s.syncBuiltinSkills(context.Background()); err == nil {
		t.Fatal("标识冲突时应停止同步，不覆盖自定义 Skill")
	}
}

func TestSkillSettingsSupportDisableAndResetWithoutChangingDefinition(t *testing.T) {
	s, mock := mockStore(t)
	item := builtins.Builtins()[0]
	for _, enabled := range []bool{false, true} {
		mock.ExpectQuery("SELECT .*FROM `ai_skill` WHERE id=.*source=").WithArgs(item.ID, "builtin", 1).WillReturnRows(skillRows(item))
		mock.ExpectExec(regexp.QuoteMeta("UPDATE `ai_skill` SET `custom_rules`=?,`enabled`=?,`updated_at`=? WHERE id=? AND source=?")).
			WithArgs("", enabled, sqlmock.AnyArg(), item.ID, "builtin").WillReturnResult(sqlmock.NewResult(0, 1))
		result, err := s.UpdateSkill(context.Background(), item.ID, enabled, "")
		if err != nil || result.Enabled != enabled || result.Content != item.Content || result.UpdatedAt.Before(time.Now().Add(-time.Minute)) {
			t.Fatalf("只应更新启停和规则: %+v %v", result, err)
		}
	}
	mock.ExpectQuery("SELECT .*FROM `ai_skill`").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	if _, err := s.UpdateSkill(context.Background(), "missing", true, ""); !errors.Is(err, common.ErrNotFound) {
		t.Fatal("不存在时不能新增 Skill")
	}
	mock.ExpectQuery("SELECT .*FROM `ai_skill`").WillReturnError(errors.New("数据库不可用"))
	if items, err := s.ListSkills(context.Background()); err == nil || len(items) != 0 {
		t.Fatal("读取失败不得回退到内置默认配置")
	}
}

func TestSkillDefinitionsAreExcludedFromHistoryCleanup(t *testing.T) {
	for _, job := range historyCleanupJobs(time.Now()) {
		if strings.Contains(job.query, "ai_skill") {
			t.Fatal("聊天和聚合到期清理不得修改 Skill 定义或配置")
		}
	}
}
