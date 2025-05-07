/*
Copyright 2025 The KodeRover Authors.

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
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/job"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

func GetJobRankMap(stages []*commonmodels.WorkflowStage) map[string]int {
	resp := make(map[string]int, 0)
	index := 0
	for _, stage := range stages {
		for _, job := range stage.Jobs {
			if !stage.Parallel {
				index++
			}
			resp[job.Name] = index
		}
		index++
	}
	return resp
}

func genJobDisplayName(jobName string, options ...string) string {
	parts := append([]string{jobName}, options...)
	return strings.Join(parts, "-")
}

func genJobKey(jobName string, options ...string) string {
	parts := append([]string{jobName}, options...)
	return strings.Join(parts, ".")
}

func GenJobName(workflow *commonmodels.WorkflowV4, jobName string, subTaskID int) string {
	stageName := ""
	stageIndex := 0
	jobIndex := 0
	for i, stage := range workflow.Stages {
		for j, job := range stage.Jobs {
			if job.Name == jobName {
				stageName = stage.Name
				stageIndex = i
				jobIndex = j
				break
			}
		}
	}

	_ = stageName

	return fmt.Sprintf("job-%d-%d-%d-%s", stageIndex, jobIndex, subTaskID, jobName)
}

func applyKeyVals(base, input commonmodels.RuntimeKeyValList, useInputKVSource bool) commonmodels.RuntimeKeyValList {
	resp := make([]*commonmodels.RuntimeKeyVal, 0)

	inputMap := make(map[string]*commonmodels.RuntimeKeyVal)
	for _, inputKV := range input {
		inputMap[inputKV.Key] = inputKV
	}

	for _, baseKV := range base {
		newKV := &commonmodels.KeyVal{
			Key:               baseKV.Key,
			Value:             baseKV.Value,
			Type:              baseKV.Type,
			IsCredential:      baseKV.IsCredential,
			ChoiceOption:      baseKV.ChoiceOption,
			ChoiceValue:       baseKV.ChoiceValue,
			Description:       baseKV.Description,
			FunctionReference: baseKV.FunctionReference,
			CallFunction:      baseKV.CallFunction,
			Script:            baseKV.Script,
		}
		item := &commonmodels.RuntimeKeyVal{
			KeyVal: newKV,
			Source: baseKV.Source,
		}

		if inputKV, ok := inputMap[baseKV.Key]; ok {
			if useInputKVSource {
				item.Source = inputKV.Source
			}

			// if the final source of the item is fix or reference, the input is irrelevant, just use the origin stuff
			if (item.Source != config.ParamSourceFixed && item.Source != config.ParamSourceReference) || useInputKVSource {
				if item.Type == commonmodels.MultiSelectType {
					item.ChoiceValue = inputKV.ChoiceValue
					// TODO: move this logic to somewhere else
					if inputKV.Value == "" {
						item.Value = strings.Join(item.ChoiceValue, ",")
					} else {
						item.Value = inputKV.Value
					}
				} else {
					// always use origin credential config.
					item.Value = inputKV.Value
				}
			}
		}

		resp = append(resp, item)
	}

	return resp
}

func applyRepos(base, input []*types.Repository) []*types.Repository {
	resp := make([]*types.Repository, 0)
	customRepoMap := make(map[string]*types.Repository)
	for _, repo := range input {
		if repo.RepoNamespace == "" {
			repo.RepoNamespace = repo.RepoOwner
		}
		customRepoMap[repo.GetKey()] = repo
	}
	for _, repo := range base {
		item := new(types.Repository)
		_ = util.DeepCopy(item, repo)
		if item.RepoNamespace == "" {
			item.RepoNamespace = item.RepoOwner
		}
		// user can only set default branch in custom workflow.
		if cv, ok := customRepoMap[repo.GetKey()]; ok {
			item.Branch = cv.Branch
			item.Tag = cv.Tag
			item.PR = cv.PR
			item.PRs = cv.PRs
			item.FilterRegexp = cv.FilterRegexp
		}

		resp = append(resp, item)
	}
	return resp
}

func renderRepos(base []*types.Repository, kvs []*commonmodels.KeyVal) {
	for _, repo := range base {
		repo.CheckoutPath = commonutil.RenderEnv(repo.CheckoutPath, kvs)
		if repo.RemoteName == "" {
			repo.RemoteName = "origin"
		}
	}
}

func renderReferredRepo(repos []*types.Repository, params []*commonmodels.Param) ([]*types.Repository, error) {
	resp := make([]*types.Repository, 0)
	repoParamMap := make(map[string]*types.Repository)
	for _, param := range params {
		if param.ParamsType == "repo" {
			repoParamMap[param.Name] = param.Repo
		}
	}

	for _, repo := range repos {
		if repo.SourceFrom != "param" {
			resp = append(resp, repo)
		} else {
			// if a repo has a source from parameter, we find the parameter. if not found return an error
			if paramRepo, ok := repoParamMap[repo.GlobalParamName]; ok {
				resp = append(resp, paramRepo)
			} else {
				return nil, fmt.Errorf("failed to find repo referring to parameter: %s", repo.GlobalParamName)
			}
		}
	}

	return resp, nil
}

func getOriginJobName(workflow *commonmodels.WorkflowV4, jobName string) (serviceReferredJob string) {
	serviceReferredJob = getOriginJobNameByRecursion(workflow, jobName, 0)
	return
}

func getOriginJobNameByRecursion(workflow *commonmodels.WorkflowV4, jobName string, depth int) (serviceReferredJob string) {
	serviceReferredJob = jobName
	// Recursion depth limit to 10
	if depth > 10 {
		return
	}
	depth++
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.Name != jobName {
				continue
			}

			switch v := job.Spec.(type) {
			case commonmodels.ZadigDistributeImageJobSpec:
				if v.Source == config.SourceFromJob {
					return getOriginJobNameByRecursion(workflow, v.JobName, depth)
				}
			case *commonmodels.ZadigDistributeImageJobSpec:
				if v.Source == config.SourceFromJob {
					return getOriginJobNameByRecursion(workflow, v.JobName, depth)
				}
			case commonmodels.ZadigDeployJobSpec:
				if v.Source == config.SourceFromJob {
					return getOriginJobNameByRecursion(workflow, v.JobName, depth)
				}
			case *commonmodels.ZadigDeployJobSpec:
				if v.Source == config.SourceFromJob {
					return getOriginJobNameByRecursion(workflow, v.JobName, depth)
				}
			case *commonmodels.ZadigVMDeployJobSpec:
				if v.Source == config.SourceFromJob {
					return getOriginJobNameByRecursion(workflow, v.JobName, depth)
				}
			case *commonmodels.ZadigScanningJobSpec:
				if v.Source == config.SourceFromJob {
					return getOriginJobNameByRecursion(workflow, v.JobName, depth)
				}
			}

		}
	}
	return
}

func getShareStorageDetail(shareStorages []*commonmodels.ShareStorage, shareStorageInfo *commonmodels.ShareStorageInfo, workflowName string, taskID int64) []*commonmodels.StorageDetail {
	resp := []*commonmodels.StorageDetail{}
	if shareStorageInfo == nil {
		return resp
	}
	if !shareStorageInfo.Enabled {
		return resp
	}
	if len(shareStorages) == 0 || len(shareStorageInfo.ShareStorages) == 0 {
		return resp
	}
	storageMap := make(map[string]*commonmodels.ShareStorage, len(shareStorages))
	for _, shareStorage := range shareStorages {
		storageMap[shareStorage.Name] = shareStorage
	}
	for _, storageInfo := range shareStorageInfo.ShareStorages {
		storage, ok := storageMap[storageInfo.Name]
		if !ok {
			continue
		}
		storageDetail := &commonmodels.StorageDetail{
			Name:      storageInfo.Name,
			Type:      types.NFSMedium,
			SubPath:   types.GetShareStorageSubPath(workflowName, storageInfo.Name, taskID),
			MountPath: storage.Path,
		}
		resp = append(resp, storageDetail)
	}
	return resp
}

// mergeKeyVals merges kv pairs from source1 and source2, and if the key collides, use source 1's value
func mergeKeyVals(source1, source2 []*commonmodels.KeyVal) []*commonmodels.KeyVal {
	resp := make([]*commonmodels.KeyVal, 0)
	existingKVMap := make(map[string]*commonmodels.KeyVal)

	for _, src1KV := range source1 {
		if _, ok := existingKVMap[src1KV.Key]; ok {
			continue
		}
		item := &commonmodels.KeyVal{
			Key:               src1KV.Key,
			Value:             src1KV.Value,
			Type:              src1KV.Type,
			IsCredential:      src1KV.IsCredential,
			ChoiceValue:       src1KV.ChoiceValue,
			ChoiceOption:      src1KV.ChoiceOption,
			Description:       src1KV.Description,
			FunctionReference: src1KV.FunctionReference,
			CallFunction:      src1KV.CallFunction,
			Script:            src1KV.Script,
		}
		existingKVMap[src1KV.Key] = src1KV
		resp = append(resp, item)
	}

	for _, src2KV := range source2 {
		if _, ok := existingKVMap[src2KV.Key]; ok {
			continue
		}

		item := &commonmodels.KeyVal{
			Key:               src2KV.Key,
			Value:             src2KV.Value,
			Type:              src2KV.Type,
			IsCredential:      src2KV.IsCredential,
			ChoiceValue:       src2KV.ChoiceValue,
			ChoiceOption:      src2KV.ChoiceOption,
			Description:       src2KV.Description,
			FunctionReference: src2KV.FunctionReference,
			CallFunction:      src2KV.CallFunction,
			Script:            src2KV.Script,
		}
		existingKVMap[src2KV.Key] = src2KV
		resp = append(resp, item)
	}
	return resp
}

// splitReposByType split the repository by types. currently it will return non-perforce repos and perforce repos
func splitReposByType(repos []*types.Repository) (gitRepos, p4Repos []*types.Repository) {
	gitRepos = make([]*types.Repository, 0)
	p4Repos = make([]*types.Repository, 0)

	for _, repo := range repos {
		if repo.Source == types.ProviderPerforce {
			p4Repos = append(p4Repos, repo)
		} else {
			gitRepos = append(gitRepos, repo)
		}
	}

	return
}

func replaceWrapLine(script string) string {
	return strings.Replace(strings.Replace(
		script,
		"\r\n",
		"\n",
		-1,
	), "\r", "\n", -1)
}

// generate script to save outputs variable to file
func outputScript(outputs []*commonmodels.Output, infrastructure string) []string {
	resp := []string{}
	if infrastructure == "" || infrastructure == setting.JobK8sInfrastructure {
		resp = []string{"set +ex"}
		for _, output := range outputs {
			resp = append(resp, fmt.Sprintf("echo $%s > %s", output.Name, path.Join(job.JobOutputDir, output.Name)))
		}
	}
	return resp
}

func modelToS3StepSpec(modelS3 *commonmodels.S3Storage) *step.S3 {
	resp := &step.S3{
		Ak:        modelS3.Ak,
		Sk:        modelS3.Sk,
		Endpoint:  modelS3.Endpoint,
		Bucket:    modelS3.Bucket,
		Subfolder: modelS3.Subfolder,
		Insecure:  modelS3.Insecure,
		Provider:  modelS3.Provider,
		Region:    modelS3.Region,
		Protocol:  "https",
	}
	if modelS3.Insecure {
		resp.Protocol = "http"
	}
	return resp
}

// generateKeyValsFromWorkflowParam generates kv from workflow parameters, ditching all parameters of repo type
func generateKeyValsFromWorkflowParam(params []*commonmodels.Param) []*commonmodels.KeyVal {
	resp := make([]*commonmodels.KeyVal, 0)

	for _, param := range params {
		if param.ParamsType == "repo" {
			continue
		}

		resp = append(resp, &commonmodels.KeyVal{
			Key:               param.Name,
			Value:             param.Value,
			Type:              commonmodels.ParameterSettingType(param.ParamsType),
			RegistryID:        "",
			ChoiceOption:      param.ChoiceOption,
			ChoiceValue:       param.ChoiceValue,
			Script:            "",
			CallFunction:      "",
			FunctionReference: nil,
			IsCredential:      param.IsCredential,
			Description:       param.Description,
		})
	}

	return resp
}

func repoNameToRepoIndex(repoName string) string {
	words := map[rune]string{
		'0': "A", '1': "B", '2': "C", '3': "D", '4': "E",
		'5': "F", '6': "G", '7': "H", '8': "I", '9': "J",
	}
	result := ""
	for i, digit := range repoName {
		if word, ok := words[digit]; ok {
			result += word
		} else {
			result += repoName[i:]
			break
		}
	}

	result = strings.Replace(result, "-", "_", -1)
	result = strings.Replace(result, ".", "_", -1)

	return result
}

func getReposVariables(repos []*types.Repository) []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	for index, repo := range repos {
		repoNameIndex := fmt.Sprintf("REPONAME_%d", index)
		ret = append(ret, &commonmodels.KeyVal{Key: repoNameIndex, Value: repo.RepoName, IsCredential: false})

		repoIndex := fmt.Sprintf("REPO_%d", index)
		repoName := repoNameToRepoIndex(repo.RepoName)
		ret = append(ret, &commonmodels.KeyVal{Key: repoIndex, Value: repoName, IsCredential: false})

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

func getEnvFromCommitMsg(commitMsg string) []*commonmodels.KeyVal {
	resp := make([]*commonmodels.KeyVal, 0)
	if commitMsg == "" {
		return resp
	}
	compileRegex := regexp.MustCompile(`(?U)#(\w+=.+)#`)
	kvArrs := compileRegex.FindAllStringSubmatch(commitMsg, -1)
	for _, kvArr := range kvArrs {
		if len(kvArr) == 0 {
			continue
		}
		keyValStr := kvArr[len(kvArr)-1]
		keyValArr := strings.Split(keyValStr, "=")
		if len(keyValArr) == 2 {
			resp = append(resp, &commonmodels.KeyVal{Key: keyValArr[0], Value: keyValArr[1], Type: commonmodels.StringType})
		}
	}
	return resp
}

// prepareDefaultWorkflowTaskEnvs System level default environment variables (every workflow type will have it)
func prepareDefaultWorkflowTaskEnvs(projectKey, workflowName, workflowDisplayName, infrastructure string, taskID int64) []*commonmodels.KeyVal {
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

	url := getTaskLink(configbase.SystemAddress(), projectKey, workflowName, workflowDisplayName, taskID)

	envs = append(envs, &commonmodels.KeyVal{Key: "TASK_URL", Value: url})
	envs = append(envs, &commonmodels.KeyVal{Key: "TASK_ID", Value: strconv.FormatInt(taskID, 10)})

	return envs
}

func getTaskLink(baseURI, projectKey, workflowName, workflowDisplayName string, taskID int64) string {
	return fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", baseURI, projectKey, workflowName, taskID, url.QueryEscape(workflowDisplayName))
}

// setRepoInfo
func setRepoInfo(repos []*types.Repository) error {
	var wg sync.WaitGroup
	for _, repo := range repos {
		wg.Add(1)
		go func(repo *types.Repository) {
			defer wg.Done()
			_ = commonservice.FillRepositoryInfo(repo)
		}(repo)
	}

	wg.Wait()
	return nil
}

var (
	outputNameRegex = regexp.MustCompile("^[a-zA-Z0-9_]{1,64}$")
)

func checkOutputNames(outputs []*commonmodels.Output) error {
	for _, output := range outputs {
		if match := outputNameRegex.MatchString(output.Name); !match {
			return fmt.Errorf("output name must match %s", "^[a-zA-Z0-9_]{1,64}$")
		}
	}
	return nil
}

// replaceServiceAndModules replaces all the <SERVICE> and <MODULE> placeholders with the
func replaceServiceAndModules(envs commonmodels.KeyValList, serviceName string, serviceModule string) (commonmodels.KeyValList, error) {
	duplicatedEnvs := make([]*commonmodels.KeyVal, 0)

	err := util.DeepCopy(&duplicatedEnvs, &envs)
	if err != nil {
		return nil, err
	}

	if serviceName == "" || serviceModule == "" {
		return duplicatedEnvs, nil
	}

	for _, env := range duplicatedEnvs {
		env.Value = strings.ReplaceAll(env.Value, "<SERVICE>", serviceName)
		env.Value = strings.ReplaceAll(env.Value, "<MODULE>", serviceModule)
	}

	return duplicatedEnvs, nil
}

func renderString(value, template string, inputs []*commonmodels.Param) string {
	for _, input := range inputs {
		if input.ParamsType == string(commonmodels.MultiSelectType) {
			input.Value = strings.Join(input.ChoiceValue, ",")
		}
		value = strings.ReplaceAll(value, fmt.Sprintf(template, input.Name), input.Value)
	}
	return value
}

func modelS3toS3(modelS3 *commonmodels.S3Storage) *step.S3 {
	resp := &step.S3{
		Ak:        modelS3.Ak,
		Sk:        modelS3.Sk,
		Endpoint:  modelS3.Endpoint,
		Bucket:    modelS3.Bucket,
		Subfolder: modelS3.Subfolder,
		Insecure:  modelS3.Insecure,
		Provider:  modelS3.Provider,
		Region:    modelS3.Region,
		Protocol:  "https",
	}
	if modelS3.Insecure {
		resp.Protocol = "http"
	}
	return resp
}

// renderParams merge the origin with the input, it will prioritize using inputs kv. if no kv is provided in the input, use the origin's kv
func renderParams(origin, input []*commonmodels.Param) []*commonmodels.Param {
	resp := make([]*commonmodels.Param, 0)
	for _, originParam := range origin {
		found := false
		for _, inputParam := range input {
			if originParam.Name == inputParam.Name {
				// always use origin credential config.
				newParam := &commonmodels.Param{
					Name:         originParam.Name,
					Description:  originParam.Description,
					ParamsType:   originParam.ParamsType,
					Value:        originParam.Value,
					Repo:         originParam.Repo,
					ChoiceOption: originParam.ChoiceOption,
					ChoiceValue:  originParam.ChoiceValue,
					Default:      originParam.Default,
					IsCredential: originParam.IsCredential,
					Source:       originParam.Source,
				}
				if originParam.Source != config.ParamSourceFixed {
					newParam.Value = inputParam.Value
					newParam.Repo = inputParam.Repo
					newParam.ChoiceValue = inputParam.ChoiceValue
				}
				resp = append(resp, newParam)
				found = true
				break
			}
		}
		if !found {
			resp = append(resp, originParam)
		}
	}

	return resp
}
