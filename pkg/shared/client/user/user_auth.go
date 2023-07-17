package user

import "github.com/koderover/zadig/pkg/tool/httpclient"

type AuthorizedResources struct {
	IsSystemAdmin   bool
	ProjectAuthInfo map[string]*ProjectActions
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

func (c *Client) GetUserAuthInfo(uid string) (*AuthorizedResources, error) {
	url := "/auth-info"
	resp := &AuthorizedResources{}
	queries := make(map[string]string)
	queries["uid"] = uid

	_, err := c.Get(url, httpclient.SetQueryParams(queries), httpclient.SetResult(resp))
	return resp, err
}
