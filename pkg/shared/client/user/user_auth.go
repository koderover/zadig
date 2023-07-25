package user

import (
	"errors"

	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/types"
)

type AuthorizedResources struct {
	IsSystemAdmin   bool
	ProjectAuthInfo map[string]*ProjectActions
	SystemActions   *SystemActions
	//AdditionalResource *AdditionalResources
	//SystemAuthInfo
}

type ProjectActions struct {
	IsProjectAdmin    bool
	Workflow          *WorkflowActions
	Env               *EnvActions
	ProductionEnv     *ProductionEnvActions
	Service           *ServiceActions
	ProductionService *ProductionServiceActions
	Build             *BuildActions
	Test              *TestActions
	Scanning          *ScanningActions
	Version           *VersionActions
}

type SystemActions struct {
	Project        *SystemProjectActions
	Template       *TemplateActions
	TestCenter     *TestCenterActions
	ReleaseCenter  *ReleaseCenterActions
	DeliveryCenter *DeliveryCenterActions
	DataCenter     *DataCenterActions
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

func (c *Client) GetUserAuthInfo(uid string) (*AuthorizedResources, error) {
	url := "/auth-info"
	resp := &AuthorizedResources{}
	queries := make(map[string]string)
	queries["uid"] = uid

	_, err := c.Get(url, httpclient.SetQueryParams(queries), httpclient.SetResult(resp))
	return resp, err
}

func (c *Client) CheckUserAuthInfoForCollaborationMode(uid, projectKey, resource, resourceName, action string) (bool, error) {
	url := "/collaboration-permission"
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

func (c *Client) ListAuthorizedProjects(uid string) ([]string, error) {
	url := "/authorized-project"

	resp := types.ListAuthorizedProjectResp{}

	queries := map[string]string{
		"uid": uid,
	}

	_, err := c.Get(url, httpclient.SetQueryParams(queries), httpclient.SetResult(resp))
	if err != nil {
		return []string{}, err
	}
	if len(resp.Error) > 0 {
		return []string{}, errors.New(resp.Error)
	}
	return resp.ProjectList, nil
}
