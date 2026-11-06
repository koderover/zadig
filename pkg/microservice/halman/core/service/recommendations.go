package service

import "github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"

func Recommendations(page models.PageContext, batch int, skills []models.Skill) ([]models.Recommendation, int) {
	items := []models.Recommendation{
		{
			SkillID: "workflow_diagnosis",
			Title:   "分析工作流报错",
			Prompt:  "请分析当前工作流任务的报错原因，并给出修复建议，不重跑任务。",
			Context: page,
		},
		{
			SkillID: "service_diagnosis",
			Title:   "分析服务异常",
			Prompt:  "请检查当前环境中的服务异常，先只读取状态、日志和事件。",
			Context: page,
		},
		{
			SkillID: "project_onboarding",
			Title:   "初始化项目",
			Prompt:  "请帮我创建 K8s 或 Helm 项目，使用已有部署配置和模板，先补齐参数并生成清单。",
			Context: page,
		},
		{
			SkillID: "service_onboarding",
			Title:   "批量初始化服务",
			Prompt:  "请帮我在现有项目中批量导入服务，使用已有配置，构建仅使用已有模板，先给出清单。",
			Context: page,
		},
		{SkillID: "environment_inspection", Title: "巡检环境健康状态", Prompt: "请巡检指定环境的健康状态，使用原巡检报告并给出建议，不执行修复。", Context: page},
		{SkillID: "efficiency_diagnosis", Title: "诊断项目质量和效率", Prompt: "请分析我指定项目近三十天的质量和效率，并给出优化建议，项目不明确时先补问。", Context: page},
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
