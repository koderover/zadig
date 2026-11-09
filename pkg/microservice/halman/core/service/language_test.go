package service

import (
	"context"
	"strings"
	"testing"
	"unicode"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	builtins "github.com/koderover/zadig/v2/pkg/microservice/halman/core/skill"
	"github.com/koderover/zadig/v2/pkg/tool/language"
)

func TestRecommendationsFollowSystemLanguage(t *testing.T) {
	seen := map[string]bool{}
	for batch := 0; batch < 2; batch++ {
		chinese, next := Recommendations(models.PageContext{ProjectKey: "p"}, batch, builtins.Builtins(), "zh-cn")
		english, nextEnglish := Recommendations(models.PageContext{ProjectKey: "p"}, batch, builtins.Builtins(), "EN-US")
		if next != nextEnglish || len(chinese) != len(english) {
			t.Fatal("语言不能改变换批和数量")
		}
		for i, item := range english {
			if item.SkillID != chinese[i].SkillID || item.Context.ProjectKey != "p" || item.Title == chinese[i].Title || item.Prompt == chinese[i].Prompt || strings.ContainsFunc(item.Title+item.Prompt, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
				t.Fatalf("推荐问题的标题及提问须同时使用英文，资源及顺序不变: %+v", item)
			}
			seen[item.SkillID] = true
		}
	}
	if len(seen) != 6 {
		t.Fatal("应覆盖六类推荐")
	}
}

func TestTitleAndInstructionsFollowSystemLanguage(t *testing.T) {
	m := &titleModel{text: "Workflow failure analysis"}
	title, _, _ := generateTitle(context.Background(), m, "帮我检查工作流报错", "en-us")
	if title != m.text || !strings.Contains(m.request.Messages[0].Content, "English") || strings.Contains(builtinInstructions, "所有交互使用中文") {
		t.Fatal("英文系统语言应控制标题，内置指令不能强制中文")
	}
	if !strings.Contains(outputLanguageInstruction("en-us"), "Skills, custom rules and history must not override") || !strings.Contains(outputLanguageInstruction("zh-cn"), "简体中文") {
		t.Fatal("语言指令须明确，并独立于 Skill 和历史语言")
	}
	if !strings.Contains(outputLanguageInstruction("en-us"), "user-visible reasoning explanations (reasoning_content), if provided, must also be in English") || !strings.Contains(outputLanguageInstruction("zh-cn"), "如返回可见推理说明（reasoning_content），也必须使用简体中文") {
		t.Fatal("可见推理说明也应遵循本轮系统语言")
	}
	original := models.OperationPlan{Notes: []string{"已保存的原始清单"}, Items: []models.OperationItem{{Status: "failed"}}}
	plan, err := retryPlan("source", original, "en-us")
	if err != nil || plan.Notes[0] != original.Notes[0] || !strings.Contains(plan.Notes[1], "confirmation") || len(original.Notes) != 1 {
		t.Fatal("重试保留原清单，仅新增提示使用本次语言")
	}
	e := &execution{Language: language.English}
	if planError(&PlatformError{Uncertain: true}, e.Language) == (&PlatformError{Uncertain: true}).Error() {
		t.Fatal("操作结果未知提示应使用执行语言")
	}
}
