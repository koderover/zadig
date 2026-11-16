package skill

import (
	"embed"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/language"
)

// 英文正文仅作为展示资源，中文定义仍是数据库与运行时的来源。
//
//go:embed *.en.md
var englishContent embed.FS

var englishMetadata = map[string]struct{ name, description string }{
	"workflow_diagnosis":     {"Workflow failure analysis", "Read task status and log evidence to identify failures and suggest fixes"},
	"service_diagnosis":      {"Service failure analysis", "Analyze service status, events and logs, and prepare a repair plan when explicitly requested"},
	"project_onboarding":     {"Project initialization", "Prepare a plan to create a project and services using existing YAML, Helm Charts or service templates"},
	"service_onboarding":     {"Service initialization", "Add services or create builds for existing services and prepare a plan for confirmation"},
	"environment_inspection": {"Environment inspection", "Inspect the health of the entire environment or query existing inspection history when explicitly requested"},
	"efficiency_diagnosis":   {"Efficiency diagnosis", "Use existing efficiency reports to identify quality and delivery bottlenecks in specified projects"},
}

// Display 复制展示字段，保留来源、分类值、管理员规则、启停及时间戳。
func Display(item models.Skill, locale language.Language) models.SkillView {
	result := models.SkillView{Skill: item, CategoryName: item.Category}
	if locale != language.English {
		return result
	}
	switch item.Category {
	case "发布辅助":
		result.CategoryName = "Release assistance"
	case "故障排查":
		result.CategoryName = "Troubleshooting"
	case "实施辅助":
		result.CategoryName = "Implementation assistance"
	case "巡检诊断":
		result.CategoryName = "Inspection and diagnosis"
	}
	metadata, ok := englishMetadata[item.ID]
	if item.Source != "builtin" || !ok {
		return result
	}
	content, err := englishContent.ReadFile(item.ID + ".en.md")
	if err != nil {
		return result
	}
	result.Name, result.Description, result.Content = metadata.name, metadata.description, string(content)
	return result
}
