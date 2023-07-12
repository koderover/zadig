/*
Copyright 2023 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package handler

type AuthorizedResources struct {
	IsSystemAdmin      bool
	ProjectAuthInfo    map[string]*ProjectActions
	AdditionalResource *AdditionalResources
}

type ProjectActions struct {
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
}

type EnvActions struct {
	View   bool
	Create bool
	// 配置
	EditConfig bool
	// 管理服务实例
	ManagePods bool
	Delete     bool
	Debug      bool
}

type ProductionEnvActions struct {
	View   bool
	Create bool
	// 配置
	EditConfig bool
	// 管理服务实例
	ManagePods bool
	Delete     bool
	Debug      bool
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

func generateUserAuthorizationInfo(uid string) (*AuthorizedResources, error) {
	return nil, nil
}

func generateSystemAuthorizationInfo() (*AuthorizedResources, error) {
	return &AuthorizedResources{
		IsSystemAdmin:      true,
		ProjectAuthInfo:    nil,
		AdditionalResource: nil,
	}, nil
}
