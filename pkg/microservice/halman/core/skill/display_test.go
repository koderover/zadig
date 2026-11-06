package skill

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/language"
)

func TestBuiltinDisplayFollowsLanguageAndPreservesDefinition(t *testing.T) {
	definitions := Builtins()
	if len(englishMetadata) != len(definitions) {
		t.Fatal("每个内置 Skill 都应提供英文展示资源")
	}
	for _, item := range definitions {
		item.Enabled, item.CustomRules = false, "项目名使用团队前缀，不要翻译此规则"
		item.CreatedAt, item.UpdatedAt = time.Now(), time.Now()
		for _, locale := range []string{"zh-cn", "", "unknown", "EN-US"} {
			t.Run(item.ID+"/"+locale, func(t *testing.T) {
				view := Display(item, language.Normalize(locale))
				if language.Normalize(locale) != language.English {
					if !reflect.DeepEqual(view.Skill, item) || view.CategoryName != item.Category {
						t.Fatal("中文及默认语言应返回数据库原文")
					}
					return
				}
				if view.Name == item.Name || view.Description == item.Description || view.Content == item.Content || view.CategoryName == item.Category || strings.ContainsFunc(view.Name+view.Description+view.Content+view.CategoryName, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
					t.Fatalf("展示字段均应使用完整英文资源: %s", item.ID)
				}
				for _, section := range []string{"Applicable scenarios", "Required information", "Steps", "Clarification", "Output requirements", "Completion criteria"} {
					if !strings.Contains(view.Content, "## "+section) {
						t.Fatalf("英文正文缺少章节: %s", section)
					}
				}
				for _, token := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(item.Content, -1) {
					if !strings.Contains(view.Content, token[1]) {
						t.Fatalf("技术标识不能在翻译中丢失: %s", token[1])
					}
				}
				view.Name, view.Description, view.Content = item.Name, item.Description, item.Content
				if !reflect.DeepEqual(view.Skill, item) {
					t.Fatal("展示转换不能改变分类值、规则、启停、ID 或时间戳")
				}
			})
		}
	}
}

func TestSkillDisplayFallbackAndCategories(t *testing.T) {
	for category, name := range map[string]string{
		"发布辅助": "Release assistance", "故障排查": "Troubleshooting",
		"实施辅助": "Implementation assistance", "巡检诊断": "Inspection and diagnosis", "未知分类": "未知分类",
	} {
		item := models.Skill{ID: "unknown", Source: "builtin", Name: "未知技能", Category: category, Description: "原说明", Content: "原正文"}
		view := Display(item, language.English)
		if !reflect.DeepEqual(view.Skill, item) || view.CategoryName != name {
			t.Fatal("未知内置定义回退原文，分类按独立映射展示")
		}
	}
	item := Builtins()[0]
	item.Source = "custom"
	if view := Display(item, language.English); !reflect.DeepEqual(view.Skill, item) {
		t.Fatal("其他来源不能套用同 ID 的内置翻译")
	}
}
