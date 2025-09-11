package user

import (
	"errors"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/types"
)

type AuthorizedResources struct {
	IsSystemAdmin   bool                       `json:"is_system_admin"`
	ProjectAuthInfo map[string]*ProjectActions `json:"project_auth_info"`
	SystemActions   *SystemActions             `json:"system_actions"`
}

type CollModeAuthorizedWorkflowWithVerb struct {
	ProjectWorkflowActionsMap map[string]map[string]*WorkflowActions `json:"project_workflow_actions_map"`
	Error                     string                                 `json:"error"`
}

type ProjectActions struct {
	IsProjectAdmin    bool                      `json:"is_system_admin"`
	Workflow          *WorkflowActions          `json:"workflow"`
	Env               *EnvActions               `json:"env"`
	ProductionEnv     *ProductionEnvActions     `json:"production_env"`
	Service           *ServiceActions           `json:"service"`
	ProductionService *ProductionServiceActions `json:"production_service"`
	Build             *BuildActions             `json:"build"`
	Test              *TestActions              `json:"test"`
	Scanning          *ScanningActions          `json:"scanning"`
	Version           *VersionActions           `json:"version"`
	Sprint            *SprintActions            `json:"sprint"`
	SprintTemplate    *SprintTemplateActions    `json:"sprint_template"`
	SprintWorkItem    *SprintWorkItemActions    `json:"sprint_workitem"`
}

type SystemActions struct {
	Project              *SystemProjectActions        `json:"project"`
	Template             *TemplateActions             `json:"template"`
	TestCenter           *TestCenterActions           `json:"test_center"`
	ReleaseCenter        *ReleaseCenterActions        `json:"release_center"`
	DeliveryCenter       *DeliveryCenterActions       `json:"delivery_center"`
	DataCenter           *DataCenterActions           `json:"data_center"`
	ReleasePlan          *ReleasePlanActions          `json:"release_plan"`
	BusinessDirectory    *BusinessDirectoryActions    `json:"business_directory"`
	ClusterManagement    *ClusterManagementActions    `json:"cluster_management"`
	VMManagement         *VMManagementActions         `json:"vm_management"`
	RegistryManagement   *RegistryManagementActions   `json:"registry_management"`
	S3StorageManagement  *S3StorageManagementActions  `json:"s3storage_management"`
	HelmRepoManagement   *HelmRepoManagementActions   `json:"helmrepo_management"`
	DBInstanceManagement *DBInstanceManagementActions `json:"dbinstance_management"`
	LabelManagement      *LabelManagementActions      `json:"label_management"`
}

type WorkflowActions struct {
	View    bool
	Create  bool
	Edit    bool
	Delete  bool
	Execute bool
	Debug   bool
}

type EnvActions struct {
	View   bool
	Create bool
	// 配置
	EditConfig bool
	// 管理服务实例
	ManagePods bool
	Delete     bool
	DebugPod   bool
	// 主机登录
	SSH bool
}

type ProductionEnvActions struct {
	View   bool
	Create bool
	// 配置
	EditConfig bool
	// 管理服务实例
	ManagePods bool
	Delete     bool
	DebugPod   bool
}

type ServiceActions struct {
	View   bool
	Create bool
	Edit   bool
	Delete bool
}

type ProductionServiceActions struct {
	View   bool
	Create bool
	Edit   bool
	Delete bool
}

type BuildActions struct {
	View   bool
	Create bool
	Edit   bool
	Delete bool
}

type TestActions struct {
	View    bool
	Create  bool
	Edit    bool
	Delete  bool
	Execute bool
}

type ScanningActions struct {
	View    bool
	Create  bool
	Edit    bool
	Delete  bool
	Execute bool
}

type VersionActions struct {
	View   bool
	Create bool
	Delete bool
}

type SystemProjectActions struct {
	Create bool
	Delete bool
}

type TemplateActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type TestCenterActions struct {
	View bool
}

type ReleaseCenterActions struct {
	View bool
}

type DeliveryCenterActions struct {
	ViewArtifact bool
	ViewVersion  bool
}

type DataCenterActions struct {
	ViewOverView      bool
	ViewInsight       bool
	EditInsightConfig bool
}

type ReleasePlanActions struct {
	Create     bool
	View       bool
	Edit       bool
	Delete     bool
	EditConfig bool
}

type BusinessDirectoryActions struct {
	View bool
}

type ClusterManagementActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type VMManagementActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type RegistryManagementActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type S3StorageManagementActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type HelmRepoManagementActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type DBInstanceManagementActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type LabelManagementActions struct {
	Create bool
	Edit   bool
	Delete bool
}

type SprintTemplateActions struct {
	Edit bool
}

type SprintActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type SprintWorkItemActions struct {
	Create bool
	Edit   bool
	Delete bool
}

func (c *Client) GetUserAuthInfo(uid string) (*AuthorizedResources, error) {
	url := "/authorization/auth-info"
	resp := &AuthorizedResources{}
	queries := make(map[string]string)
	queries["uid"] = uid

	_, err := c.Get(url, httpclient.SetQueryParams(queries), httpclient.SetResult(resp))
	return resp, err
}

func (c *Client) CheckUserAuthInfoForCollaborationMode(uid, projectKey, resource, resourceName, action string) (bool, error) {
	url := "/authorization/collaboration-permission"
	resp := &types.CheckCollaborationModePermissionResp{}

	queries := make(map[string]string)
	queries["uid"] = uid
	queries["project_key"] = projectKey
	queries["resource"] = resource
	queries["resource_name"] = resourceName
	queries["action"] = action

	_, err := c.Get(url, httpclient.SetQueryParams(queries), httpclient.SetResult(resp))
	if err != nil {
		return false, err
	}
	if len(resp.Error) > 0 {
		return resp.HasPermission, errors.New(resp.Error)
	}

	return resp.HasPermission, nil
}

func (c *Client) ListAuthorizedProjects(uid string) ([]string, bool, error) {
	url := "/authorization/authorized-projects"

	resp := &types.ListAuthorizedProjectResp{}

	queries := map[string]string{
		"uid": uid,
	}

	_, err := c.Get(url, httpclient.SetQueryParams(queries), httpclient.SetResult(resp))
	if err != nil {
		return []string{}, false, err
	}
	if len(resp.Error) > 0 {
		return []string{}, false, errors.New(resp.Error)
	}
	return resp.ProjectList, resp.Found, nil
}

func (c *Client) ListAuthorizedProjectsByResourceAndVerb(uid, resource, verb string) ([]string, bool, error) {
	url := "/authorization/authorized-projects/verb"

	resp := &types.ListAuthorizedProjectResp{}

	queries := map[string]string{
		"uid":      uid,
		"resource": resource,
		"verb":     verb,
	}

	_, err := c.Get(url, httpclient.SetQueryParams(queries), httpclient.SetResult(resp))
	if err != nil {
		return []string{}, false, err
	}
	if len(resp.Error) > 0 {
		return []string{}, false, errors.New(resp.Error)
	}
	return resp.ProjectList, resp.Found, nil
}

func (c *Client) ListAuthorizedWorkflows(uid, projectKey string) ([]string, []string, error) {
	url := "/authorization/authorized-workflows"

	resp := &types.ListAuthorizedWorkflowsResp{}

	queries := map[string]string{
		"uid":         uid,
		"project_key": projectKey,
	}

	_, err := c.Get(url, httpclient.SetQueryParams(queries), httpclient.SetResult(resp))
	if err != nil {
		return []string{}, []string{}, err
	}
	if len(resp.Error) > 0 {
		return []string{}, []string{}, errors.New(resp.Error)
	}
	return resp.WorkflowList, resp.CustomWorkflowList, nil
}

func (c *Client) ListAuthorizedWorkflowsWithVerb(uid, projectKey string) (*CollModeAuthorizedWorkflowWithVerb, error) {
	url := "/authorization/authorized-workflows/verb"

	resp := &CollModeAuthorizedWorkflowWithVerb{}

	queries := map[string]string{
		"uid":         uid,
		"project_key": projectKey,
	}

	res, err := c.Get(url, httpclient.SetQueryParams(queries), httpclient.SetResult(resp))
	if err != nil {
		return nil, err
	}
	if res.IsError() {
		return nil, errors.New(res.String())
	}

	if len(resp.Error) > 0 {
		return nil, errors.New(resp.Error)
	}

	return resp, nil
}

func (c *Client) ListCollaborationEnvironmentsPermission(uid, projectKey string) (*types.CollaborationEnvPermission, error) {
	url := "/authorization/authorized-envs"

	resp := &types.CollaborationEnvPermission{}

	queries := map[string]string{
		"uid":         uid,
		"project_key": projectKey,
	}

	_, err := c.Get(url, httpclient.SetQueryParams(queries), httpclient.SetResult(resp))
	if err != nil {
		return nil, err
	}
	if len(resp.Error) > 0 {
		return nil, errors.New(resp.Error)
	}
	return resp, nil
}

func (c *Client) CheckPermissionGivenByCollaborationMode(uid, projectKey, resource, action string) (bool, error) {
	url := "/authorization/collaboration-action"
	resp := &types.CheckCollaborationModePermissionResp{}

	queries := make(map[string]string)
	queries["uid"] = uid
	queries["project_key"] = projectKey
	queries["resource"] = resource
	queries["action"] = action

	_, err := c.Get(url, httpclient.SetQueryParams(queries), httpclient.SetResult(resp))
	if err != nil {
		return false, err
	}
	if len(resp.Error) > 0 {
		return resp.HasPermission, errors.New(resp.Error)
	}

	return resp.HasPermission, nil
}

type createRoleBindingReq struct {
	Role       string      `json:"role"`
	Identities []*identity `json:"identities"`
}

type identity struct {
	IdentityType string `json:"identity_type"`
	UID          string `json:"uid"`
	GID          string `json:"gid"`
}

func (c *Client) CreateUserRoleBinding(uid, namespace, roleName string) error {
	url := fmt.Sprintf("/policy/role-bindings?namespace=%s", namespace)

	req := &createRoleBindingReq{
		Role: roleName,
		Identities: []*identity{
			{
				IdentityType: "user",
				UID:          uid,
			},
		},
	}

	_, err := c.Post(url, httpclient.SetBody(req))

	return err
}
