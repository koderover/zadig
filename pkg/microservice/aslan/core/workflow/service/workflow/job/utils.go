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

	configbase "github.com/koderover/zadig/v2/pkg/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

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
