package skill

import (
	_ "embed"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

//go:embed workflow_diagnosis.md
var workflowDiagnosis string

//go:embed service_diagnosis.md
var serviceDiagnosis string

//go:embed project_onboarding.md
var projectOnboarding string

//go:embed service_onboarding.md
var serviceOnboarding string

//go:embed environment_inspection.md
var environmentInspection string

//go:embed efficiency_diagnosis.md
var efficiencyDiagnosis string

// Builtins 仅作为初始化和升级来源，运行时读取数据库中的有效定义。
func Builtins() []models.Skill {
	return []models.Skill{
		{ID: "workflow_diagnosis", Source: "builtin", Name: "工作流报错分析", Category: "故障排查", Description: "读取任务状态和日志证据，定位失败原因并给出修复建议", Content: workflowDiagnosis, Enabled: true},
		{ID: "service_diagnosis", Source: "builtin", Name: "服务报错分析", Category: "故障排查", Description: "分析服务状态、事件和日志，按明确请求准备修复清单", Content: serviceDiagnosis, Enabled: true},
		{ID: "project_onboarding", Source: "builtin", Name: "项目初始化", Category: "实施辅助", Description: "使用已有 YAML、Helm Chart 或服务模板，准备创建项目和服务的清单", Content: projectOnboarding, Enabled: true},
		{ID: "service_onboarding", Source: "builtin", Name: "服务初始化", Category: "实施辅助", Description: "批量添加服务或为已有服务创建构建，准备待确认清单", Content: serviceOnboarding, Enabled: true},
		{ID: "environment_inspection", Source: "builtin", Name: "环境巡检", Category: "巡检诊断", Description: "按用户明确请求巡检整个环境的健康状态，或查询原有巡检历史", Content: environmentInspection, Enabled: true},
		{ID: "efficiency_diagnosis", Source: "builtin", Name: "效能诊断", Category: "巡检诊断", Description: "分析效能报告，诊断指定项目的质量和效率瓶颈", Content: efficiencyDiagnosis, Enabled: true},
	}
}
