package service

import (
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/language"
)

func Recommendations(page models.PageContext, batch int, skills []models.Skill, locale string) ([]models.Recommendation, int) {
	lang := language.Normalize(locale)
	items := []models.Recommendation{
		{
			SkillID: "workflow_diagnosis",
			Title:   lang.Text("分析工作流报错", "Analyze workflow failures"),
			Prompt:  lang.Text("分析工作流任务的报错原因，并给出修复建议。", "Analyze the errors in the current workflow run and suggest fixes."),
			Context: page,
		},
		{
			SkillID: "service_diagnosis",
			Title:   lang.Text("分析服务异常", "Diagnose service issues"),
			Prompt:  lang.Text("分析服务状态与错误日志，定位服务异常根因。", "Analyze service status and error logs to identify the root cause of service issues."),
			Context: page,
		},
		{
			SkillID: "project_onboarding",
			Title:   lang.Text("初始化项目", "Initialize a project"),
			Prompt:  lang.Text("请帮我创建 K8s YAML 或 Helm 项目，并使用已有 YAML、Helm Chart 或服务模板在项目中创建服务，先补齐参数并生成清单。", "Help me create a K8s YAML or Helm project and add services using existing YAML, Helm Charts or service templates. Collect missing parameters and prepare a plan first."),
			Context: page,
		},
		{
			SkillID: "service_onboarding",
			Title:   lang.Text("批量初始化服务", "Initialize multiple services"),
			Prompt:  lang.Text("请帮我在现有项目中使用已有 YAML、Helm Chart 或服务模板批量创建服务，构建仅使用已有构建模板，先补齐参数并生成清单。", "Help me add multiple services to an existing project using existing YAML, Helm Charts or service templates. Use only existing build templates for builds. Collect missing parameters and prepare a plan first."),
			Context: page,
		},
		{
			SkillID: "environment_inspection",
			Title:   lang.Text("巡检整个环境", "Inspect the entire environment"),
			Prompt:  lang.Text("请巡检指定环境的整体健康状态，并给出修复建议。", "Inspect the overall health of the entire specified environment and suggest improvements."),
			Context: page},
		{SkillID: "efficiency_diagnosis",
			Title:   lang.Text("诊断项目质量和效率", "Analyze project quality and efficiency"),
			Prompt:  lang.Text("请分析我指定项目近三十天的质量和效率，并给出优化建议，项目不明确时先补问。", "Analyze quality and efficiency over the last thirty days for the projects I specify and suggest improvements. Ask which projects to analyze if unclear."),
			Context: page},
	}

	priority := "project_onboarding"
	if page.ProjectKey != "" {
		priority = "service_onboarding"
	}

	if page.EnvName != "" || page.ServiceName != "" || page.PageType == "environment" {
		priority = "service_diagnosis"
	}
	if page.PageType == "environment" && page.ServiceName == "" {
		priority = "environment_inspection"
	}
	if page.PageType == "efficiency" {
		priority = "efficiency_diagnosis"
	}

	if page.PageType == "workflow_task" || page.WorkflowName != "" {
		priority = "workflow_diagnosis"
	}

	for i := range items {
		if items[i].SkillID == priority {
			items[0], items[i] = items[i], items[0]
			break
		}
	}

	available := make([]models.Recommendation, 0, len(items))
	for _, item := range items {
		for _, skill := range skills {
			if skill.Source == "builtin" && skill.ID == item.SkillID && skill.Enabled {
				available = append(available, item)
				break
			}
		}
	}
	result := make([]models.Recommendation, 0, min(3, len(available)))
	if len(available) == 0 {
		return result, 0
	}
	for i := 0; i < min(3, len(available)); i++ {
		result = append(result, available[((batch%len(available))*3+i)%len(available)])
	}
	return result, (batch + 1) % len(available)
}
