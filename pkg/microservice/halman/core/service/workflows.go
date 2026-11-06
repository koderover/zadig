package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
)

func (s *AgentService) verifyDefaultWorkflows(ctx context.Context, e *execution, item models.OperationItem) (string, string, error) {
	unknown := e.text("默认工作流尚未完整核实，未调用创建接口", "Default workflows have not been fully verified; no creation request was made")
	if err := checkNames(item.ProjectKey); err != nil {
		return "unknown", unknown, err
	}
	if item.ProjectType != "k8s" && item.ProjectType != "helm" {
		return "unknown", unknown, common.ErrRequestConflict
	}
	raw, err := s.Platform.business(ctx, e.Authorization, http.MethodGet, "/api/v1/authorization/auth-info", url.Values{"uid": {e.User}}, nil)
	if err != nil {
		return "unknown", unknown, err
	}
	var permissions userclient.AuthorizedResources
	if json.Unmarshal(raw, &permissions) != nil {
		return "unknown", unknown, common.ErrRequestConflict
	}
	project := permissions.ProjectAuthInfo[item.ProjectKey]
	// 原列表可能静默过滤无权查看的条目，核实完整集合时不能把可见子集当成全部。
	if !permissions.IsSystemAdmin && (project == nil || (!project.IsProjectAdmin &&
		(project.Env == nil || !project.Env.View || project.Workflow == nil || !project.Workflow.View))) {
		return "unknown", e.text("缺少完整环境及工作流查看权限，保留待核实", "Full environment and workflow view permissions are required; verification remains pending"), nil
	}
	raw, err = s.Platform.business(ctx, e.Authorization, http.MethodGet, "/api/environment/environments", url.Values{"projectName": {item.ProjectKey}, "production": {"false"}}, nil)
	if err != nil {
		return "unknown", unknown, err
	}
	var envs []struct {
		Project string `json:"product_name"`
		Name    string `json:"env_name"`
		IsProd  bool   `json:"is_prod"`
	}
	if json.Unmarshal(raw, &envs) != nil {
		return "unknown", unknown, common.ErrRequestConflict
	}
	expected := map[string]bool{item.ProjectKey + "-ops-workflow": false}
	for _, env := range envs {
		if env.Project != item.ProjectKey || env.Name == "" || env.IsProd {
			return "unknown", unknown, common.ErrRequestConflict
		}
		expected[item.ProjectKey+"-workflow-"+env.Name] = false
	}
	raw, err = s.Platform.business(ctx, e.Authorization, http.MethodGet, "/api/workflow/v4", url.Values{"project": {item.ProjectKey}}, nil)
	if err != nil {
		return "unknown", unknown, err
	}
	var workflows struct {
		Items []struct {
			Name    string `json:"name"`
			Project string `json:"projectName"`
			Type    string `json:"workflow_type"`
		} `json:"workflow_list"`
		Total int `json:"total"`
	}
	if json.Unmarshal(raw, &workflows) != nil {
		return "unknown", unknown, common.ErrRequestConflict
	}
	if workflows.Total != len(workflows.Items) {
		return "unknown", e.text("工作流列表不完整，保留待核实", "The workflow list is incomplete; verification remains pending"), nil
	}
	for _, workflow := range workflows.Items {
		if _, wanted := expected[workflow.Name]; wanted && workflow.Project == item.ProjectKey && workflow.Type == setting.CustomWorkflowType {
			expected[workflow.Name] = true
		}
	}
	for _, found := range expected {
		if !found {
			return "unknown", unknown, nil
		}
	}
	return "succeeded", e.text("已有列表中的默认工作流名称及项目归属已核实，未重复创建", "Default workflow names and project ownership were verified in the existing lists; nothing was recreated"), nil
}
