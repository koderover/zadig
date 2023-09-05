/*
Copyright 2021 The KodeRover Authors.

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

package permission

import (
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
	"go.uber.org/zap"
)

type ResourceDefinition struct {
	Resource string    `json:"resource"`
	Alias    string    `json:"alias"`
	Rules    []*Action `json:"rules"`
}

type Action struct {
	Action string `json:"action"`
	Alias  string `json:"alias"`
}

var systemResourceActionAliasMap = map[string]string{
	"Project":            "项目",
	"Template":           "模板库",
	"ReleasePlan":        "发布计划",
	"QualityCenter":      "质量中心",
	"ArtifactManagement": "制品管理",
	"ProjectView":        "业务目录",
	"DataCenter":         "数据视图",
}

var projectResourceAliasMap = map[string]string{
	"Workflow":              "工作流",
	"Environment":           "测试环境",
	"ProductionEnvironment": "生产环境",
	"Service":               "测试服务",
	"ProductionService":     "生产服务",
	"Build":                 "构建",
	"Test":                  "测试",
	"Scan":                  "代码扫描",
	"Delivery":              "版本管理",
}

func GetResourceActionDefinitions(scope, envType string, log *zap.SugaredLogger) ([]*ResourceDefinition, error) {
	var dbActionType int
	switch scope {
	case string(types.SystemScope):
		dbActionType = types.DBSystemScope
	case string(types.ProjectScope):
		dbActionType = types.DBProjectScope
	}

	actionList, err := orm.ListActionByType(dbActionType, repository.DB)
	if err != nil {
		log.Errorf("failed to list action with type: %s, error: %s", scope, err)
		return nil, fmt.Errorf("failed to list action with type: %s, error: %s", scope, err)
	}

	resourceMap := make(map[string]*ResourceDefinition)
	for _, action := range actionList {
		if _, ok := resourceMap[action.Resource]; !ok {
			alias := projectResourceAliasMap[action.Resource]
			if scope == string(types.SystemScope) {
				alias = systemResourceActionAliasMap[action.Resource]
			}
			resourceMap[action.Resource] = &ResourceDefinition{
				Resource: action.Resource,
				Alias:    alias,
				Rules:    make([]*Action, 0),
			}
		}

		// there are special case where we will just skip
		// 1. when envType is k8s, we don't need ssh_pm for environment (production env doesn't have this action)
		// 2. when envType is pm, we don't need debug_pod for both environment and production env
		if envType == setting.PMDeployType {
			if action.Action == VerbDebugEnvironmentPod || action.Action == VerbDebugProductionEnvPod {
				continue
			}
		}
		if envType == setting.K8SDeployType {
			if action.Action == VerbEnvironmentSSHPM {
				continue
			}
		}
		resourceMap[action.Resource].Rules = append(resourceMap[action.Resource].Rules, &Action{
			Action: action.Action,
			Alias:  action.Name,
		})
	}

	resp := make([]*ResourceDefinition, 0)
	for _, def := range resourceMap {
		resp = append(resp, def)
	}

	return resp, nil
}
