package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	builtins "github.com/koderover/zadig/v2/pkg/microservice/halman/core/skill"
)

func expectSkillSnapshot(mock sqlmock.Sqlmock) {
	rows := sqlmock.NewRows([]string{"id", "source", "name", "content", "enabled", "custom_rules"})
	for _, item := range builtins.Builtins() {
		rows.AddRow(item.ID, item.Source, item.Name, item.Content, item.Enabled, item.CustomRules)
	}
	mock.ExpectQuery("SELECT .*FROM `ai_skill` WHERE source=").WithArgs("builtin").WillReturnRows(rows)
}

func TestSkillRulesUseSnapshotAndDisabledToolsRejectNewTasks(t *testing.T) {
	skills := builtins.Builtins()
	skills[0].Content, skills[0].CustomRules = "数据库中的诊断正文", "先输出证据"
	skills[1].Enabled = false
	e := &execution{Skills: skills}
	text := skillInstructions(e)
	if !strings.Contains(text, "数据库中的诊断正文") || !strings.Contains(text, "先输出证据") || strings.Contains(text, skills[1].Content) {
		t.Fatal("应按数据库快照组合可用 Skill 正文和规则")
	}
	if _, err := (&AgentService{}).skillExecution(context.WithValue(context.Background(), executionKey{}, e), "service_diagnosis"); err == nil {
		t.Fatal("直接调用工具也必须拒绝已停用的新任务")
	}
	e.ContinuingSkill, e.SkillID = "service_diagnosis", "service_diagnosis"
	if _, err := (&AgentService{}).skillExecution(context.WithValue(context.Background(), executionKey{}, e), "service_diagnosis"); err != nil {
		t.Fatalf("已经开始的合法任务仍可调用: %v", err)
	}
	if !strings.Contains(skillInstructions(e), skills[1].Content) {
		t.Fatal("继续已有任务时仍应加载该 Skill 规则")
	}
}

func TestSkillSnapshotValidatesContinuationBeforeAllowingDisabledSkill(t *testing.T) {
	for _, state := range []string{"awaiting_input", "pending_confirmation", "succeeded", "cancelled"} {
		t.Run(state, func(t *testing.T) {
			store, mock := mockStore(t)
			expectSkillSnapshot(mock)
			mock.ExpectQuery("SELECT r.id,r.conversation_id,r.created_at").WillReturnRows(sqlmock.NewRows([]string{"id", "conversation_id", "created_at", "skill_id", "skill_execution_id", "skill_status"}).
				AddRow("root", "conversation", time.Now(), "project_onboarding", "root", state))
			s := &AgentService{Store: store}
			_, continuing, err := s.skillSnapshot(context.Background(), "user", "conversation", models.RunRequest{SkillExecutionID: "root"}, time.Now().Add(-time.Hour))
			if state == "awaiting_input" || state == "pending_confirmation" {
				if err != nil || continuing != "project_onboarding" {
					t.Fatalf("合法补问应允许继续: %s %v", continuing, err)
				}
			} else if !errors.Is(err, common.ErrRequestConflict) {
				t.Fatal("不能借用已结束的根执行发起新任务")
			}
		})
	}
}

func TestRecommendationsFilterDisabledSkillsWithoutDuplicates(t *testing.T) {
	for count := 0; count <= len(builtins.Builtins()); count++ {
		skills := builtins.Builtins()
		for i := range skills {
			skills[i].Enabled = i < count
		}
		for batch := 0; batch < 5; batch++ {
			items, next := Recommendations(models.PageContext{}, batch, skills)
			if len(items) != min(3, count) || (count == 0 && next != 0) || (count > 0 && next != (batch+1)%count) {
				t.Fatalf("数量或轮换不正确: %d %d %+v %d", count, batch, items, next)
			}
			seen := map[string]bool{}
			for _, item := range items {
				if seen[item.SkillID] || !skillAllowed(&execution{Skills: skills}, item.SkillID) || item.Prompt == "" {
					t.Fatal("推荐必须启用、不重复并保留原提问")
				}
				seen[item.SkillID] = true
			}
		}
	}
}
