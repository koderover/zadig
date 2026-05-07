package util

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

func BuildPayloadVariables(rawPayload string) []*commonmodels.KeyVal {
	if rawPayload == "" {
		return nil
	}

	var payload interface{}
	if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		return nil
	}

	resp := make([]*commonmodels.KeyVal, 0)
	flattenPayloadValue("payload", payload, &resp)
	return resp
}

func flattenPayloadValue(prefix string, value interface{}, resp *[]*commonmodels.KeyVal) {
	switch val := value.(type) {
	case map[string]interface{}:
		for key, item := range val {
			flattenPayloadValue(prefix+"."+key, item, resp)
		}
	case []interface{}:
		for index, item := range val {
			flattenPayloadValue(fmt.Sprintf("%s.%d", prefix, index), item, resp)
		}
	case string:
		*resp = append(*resp, &commonmodels.KeyVal{Key: prefix, Value: val, IsCredential: false})
	case float64:
		*resp = append(*resp, &commonmodels.KeyVal{Key: prefix, Value: strconv.FormatFloat(val, 'f', -1, 64), IsCredential: false})
	case bool:
		*resp = append(*resp, &commonmodels.KeyVal{Key: prefix, Value: strconv.FormatBool(val), IsCredential: false})
	case nil:
		return
	default:
		*resp = append(*resp, &commonmodels.KeyVal{Key: prefix, Value: fmt.Sprint(val), IsCredential: false})
	}
}

func RepoVariableKVs(repos []*types.Repository) []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	for index, repo := range repos {
		repoNameIndex := fmt.Sprintf("REPONAME_%d", index)
		ret = append(ret, &commonmodels.KeyVal{Key: repoNameIndex, Value: repo.RepoName, IsCredential: false})

		repoIndex := fmt.Sprintf("REPO_%d", index)
		repoName := RepoNameToRepoIndex(repo.RepoName)
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

		ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_PRE_MERGE_BRANCHES", repoName), Value: repo.GetPreMergeBranches(), IsCredential: false})
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
		if len(repo.AuthorName) > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_AUTHOR", repoName), Value: repo.AuthorName, IsCredential: false})
		}
		if len(repo.Committer) > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_COMMITTER", repoName), Value: repo.Committer, IsCredential: false})
		}
		if len(repo.CommitMessage) > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_COMMIT_MESSAGE", repoName), Value: repo.CommitMessage, IsCredential: false})
		}
		if len(repo.TargetBranch) > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_TARGET_BRANCH", repoName), Value: repo.TargetBranch, IsCredential: false})
		}
	}
	return ret
}

func RepoNameToRepoIndex(repoName string) string {
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

	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ReplaceAll(result, ".", "_")
	return result
}

func CollectWorkflowRepos(workflow *commonmodels.WorkflowV4) []*types.Repository {
	if workflow == nil {
		return nil
	}

	resp := make([]*types.Repository, 0)
	repoKeySet := make(map[string]struct{})
	appendRepo := func(repo *types.Repository) {
		if repo == nil {
			return
		}
		key := fmt.Sprintf("%d/%s/%s/%s/%s/%d", repo.CodehostID, repo.RepoOwner, repo.RepoNamespace, repo.RepoName, repo.Branch, repo.PR)
		if _, ok := repoKeySet[key]; ok {
			return
		}
		repoKeySet[key] = struct{}{}
		resp = append(resp, repo)
	}

	for _, stage := range workflow.Stages {
		for _, jobInfo := range stage.Jobs {
			switch spec := jobInfo.Spec.(type) {
			case *commonmodels.ZadigBuildJobSpec:
				for _, build := range spec.ServiceAndBuilds {
					for _, repo := range build.Repos {
						appendRepo(repo)
					}
				}
			case *commonmodels.ZadigTestingJobSpec:
				for _, testModule := range spec.TestModules {
					for _, repo := range testModule.Repos {
						appendRepo(repo)
					}
				}
				for _, serviceTest := range spec.ServiceAndTests {
					for _, repo := range serviceTest.Repos {
						appendRepo(repo)
					}
				}
			case *commonmodels.ZadigScanningJobSpec:
				for _, scanning := range spec.Scannings {
					for _, repo := range scanning.Repos {
						appendRepo(repo)
					}
				}
				for _, serviceScanning := range spec.ServiceAndScannings {
					for _, repo := range serviceScanning.Repos {
						appendRepo(repo)
					}
				}
			}
		}
	}

	return resp
}

func BuildWorkflowSystemVariableKVs(workflow *commonmodels.WorkflowV4, projectName, projectDisplayName string, taskID int64, creator, account, uid string, now time.Time) []*commonmodels.KeyVal {
	if workflow == nil {
		return nil
	}

	resp := []*commonmodels.KeyVal{
		{Key: "project", Value: projectName, IsCredential: false},
		{Key: "project.id", Value: projectName, IsCredential: false},
		{Key: "project.name", Value: projectDisplayName, IsCredential: false},
		{Key: "workflow.id", Value: workflow.Name, IsCredential: false},
		{Key: "workflow.name", Value: workflow.DisplayName, IsCredential: false},
		{Key: "workflow.task.id", Value: fmt.Sprintf("%d", taskID), IsCredential: false},
		{Key: "workflow.task.creator", Value: creator, IsCredential: false},
		{Key: "workflow.task.creator.id", Value: account, IsCredential: false},
		{Key: "workflow.task.creator.userId", Value: uid, IsCredential: false},
		{Key: "workflow.task.timestamp", Value: fmt.Sprintf("%d", now.Unix()), IsCredential: false},
		{Key: "workflow.task.datetime", Value: now.Format(time.DateTime), IsCredential: false},
		{
			Key:          "workflow.task.url",
			Value:        fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", configbase.SystemAddress(), projectName, workflow.Name, taskID, url.QueryEscape(workflow.DisplayName)),
			IsCredential: false,
		},
	}

	for _, param := range workflow.Params {
		if param == nil {
			continue
		}
		value := param.Value
		if param.ParamsType == string(commonmodels.MultiSelectType) {
			value = strings.Join(param.ChoiceValue, ",")
		} else if param.ParamsType == string(commonmodels.FileType) {
			continue
		}
		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"workflow", "params", param.Name}, "."),
			Value:        value,
			IsCredential: false,
		})
	}
	if workflow.HookPayload != nil {
		resp = append(resp, BuildPayloadVariables(workflow.HookPayload.RawPayload)...)
	}

	return resp
}

func BuildWorkflowRuntimeVariableKVs(workflow *commonmodels.WorkflowV4, projectName, projectDisplayName string, taskID int64, creator, account, uid string, now time.Time) []*commonmodels.KeyVal {
	resp := BuildWorkflowSystemVariableKVs(workflow, projectName, projectDisplayName, taskID, creator, account, uid, now)
	if workflow == nil {
		return resp
	}
	resp = append(resp, RepoVariableKVs(CollectWorkflowRepos(workflow))...)

	return resp
}

func KeyValsToMap(kvs []*commonmodels.KeyVal) map[string]string {
	resp := make(map[string]string)
	for _, kv := range kvs {
		if kv == nil || kv.Key == "" || kv.GetValue() == "" {
			continue
		}
		resp[kv.Key] = kv.GetValue()
	}
	return resp
}
