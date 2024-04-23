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

package job

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/util/sets"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

func FilterServiceVars(serviceName string, deployContents []config.DeployContent, service *commonmodels.DeployServiceInfo, serviceEnv *commonservice.EnvService) (*commonmodels.DeployServiceInfo, error) {
	if serviceEnv == nil {
		return service, fmt.Errorf("service: %v do not exist", serviceName)
	}
	defaultUpdateConfig := false
	if slices.Contains(deployContents, config.DeployConfig) && serviceEnv.Updatable {
		defaultUpdateConfig = true
	}

	keySet := sets.NewString()
	if service == nil {
		service = &commonmodels.DeployServiceInfo{}
	} else {
		for _, config := range service.VariableConfigs {
			keySet = keySet.Insert(config.VariableKey)
		}
	}

	service.VariableYaml = serviceEnv.VariableYaml
	service.ServiceName = serviceName
	service.Updatable = serviceEnv.Updatable
	service.UpdateConfig = defaultUpdateConfig

	service.VariableKVs = []*commontypes.RenderVariableKV{}
	service.LatestVariableKVs = []*commontypes.RenderVariableKV{}

	for _, svcVar := range serviceEnv.VariableKVs {
		if keySet.Has(svcVar.Key) && !svcVar.UseGlobalVariable {
			service.VariableKVs = append(service.VariableKVs, svcVar)
		}
	}
	for _, svcVar := range serviceEnv.LatestVariableKVs {
		if keySet.Has(svcVar.Key) && !svcVar.UseGlobalVariable {
			service.LatestVariableKVs = append(service.LatestVariableKVs, svcVar)
		}
	}
	if !slices.Contains(deployContents, config.DeployVars) {
		service.VariableKVs = []*commontypes.RenderVariableKV{}
		service.LatestVariableKVs = []*commontypes.RenderVariableKV{}
	}

	return service, nil
}

// PrepareDefaultWorkflowTaskEnvs System level default environment variables (every workflow type will have it)
func PrepareDefaultWorkflowTaskEnvs(projectKey, workflowName, workflowDisplayName, infrastructure string, taskID int64) []*commonmodels.KeyVal {
	envs := make([]*commonmodels.KeyVal, 0)

	envs = append(envs,
		&commonmodels.KeyVal{Key: "CI", Value: "true"},
		&commonmodels.KeyVal{Key: "ZADIG", Value: "true"},
		&commonmodels.KeyVal{Key: "PROJECT", Value: projectKey},
		&commonmodels.KeyVal{Key: "WORKFLOW", Value: workflowName},
	)

	if infrastructure != setting.JobVMInfrastructure {
		envs = append(envs, &commonmodels.KeyVal{Key: "WORKSPACE", Value: "/workspace"})
	}

	url := GetLink(configbase.SystemAddress(), projectKey, workflowName, workflowDisplayName, taskID)

	envs = append(envs, &commonmodels.KeyVal{Key: "TASK_URL", Value: url})
	envs = append(envs, &commonmodels.KeyVal{Key: "TASK_ID", Value: strconv.FormatInt(taskID, 10)})

	return envs
}

func GetLink(baseURI, projectKey, workflowName, workflowDisplayName string, taskID int64) string {
	return fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", baseURI, projectKey, workflowName, taskID, url.QueryEscape(workflowDisplayName))
}

func getReposVariables(repos []*types.Repository) []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	for index, repo := range repos {
		repoNameIndex := fmt.Sprintf("REPONAME_%d", index)
		ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf(repoNameIndex), Value: repo.RepoName, IsCredential: false})

		repoName := strings.Replace(repo.RepoName, "-", "_", -1)
		repoName = strings.Replace(repoName, ".", "_", -1)

		repoIndex := fmt.Sprintf("REPO_%d", index)
		ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf(repoIndex), Value: repoName, IsCredential: false})

		if len(repo.Branch) > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_BRANCH", repoName), Value: repo.Branch, IsCredential: false})
		}

		if len(repo.Tag) > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_TAG", repoName), Value: repo.Tag, IsCredential: false})
		}

		if repo.PR > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_PR", repoName), Value: strconv.Itoa(repo.PR), IsCredential: false})
		}

		ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_ORG", repoName), Value: repo.RepoOwner, IsCredential: false})

		if len(repo.PRs) > 0 {
			prStrs := []string{}
			for _, pr := range repo.PRs {
				prStrs = append(prStrs, strconv.Itoa(pr))
			}
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_PR", repoName), Value: strings.Join(prStrs, ","), IsCredential: false})
		}

		if len(repo.CommitID) > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_COMMIT_ID", repoName), Value: repo.CommitID, IsCredential: false})
		}
		ret = append(ret, getEnvFromCommitMsg(repo.CommitMessage)...)
	}
	return ret
}
