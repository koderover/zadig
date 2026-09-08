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

package workflow

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	codehostmodels "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

var _ = Describe("Testing utils", func() {

	Context("validateHookNames", func() {
		It("should be passed for valid names", func() {
			err := validateHookNames([]string{"a"})
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("should raise error for empty name", func() {
			err := validateHookNames([]string{"a", ""})
			Expect(err).Should(HaveOccurred())
		})
		It("should raise error for invalid characters", func() {
			err := validateHookNames([]string{"a", "*"})
			Expect(err).Should(HaveOccurred())
		})
		It("should raise error for duplicated names", func() {
			err := validateHookNames([]string{"a", "a"})
			Expect(err).Should(HaveOccurred())
		})
	})
})

func TestValidateFrontendWorkflowDeltaJobExecutePolicy(t *testing.T) {
	base := &commonmodels.WorkflowV4{
		Stages: []*commonmodels.WorkflowStage{
			{
				Name: "stage-1",
				Jobs: []*commonmodels.Job{
					{
						Name: "job-1",
						ExecutePolicy: &commonmodels.JobExecutePolicy{
							Type:      config.JobExecutePolicyTypeExecute,
							MatchRule: config.JobExecutePolicyMatchRuleAll,
						},
					},
				},
			},
		},
	}

	t.Run("rejects a direct execute policy change", func(t *testing.T) {
		patches := []*commonmodels.JSONPatchOperation{
			{
				Operation: "replace",
				Path:      "/stages/0/jobs/0/execute_policy/type",
				Value:     config.JobExecutePolicyTypeSkip,
			},
		}

		_, _, err := validateFrontendWorkflowDelta(base, patches)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "execute_policy must be consistent with workflow template")
	})

	t.Run("rejects an execute policy change through job replacement", func(t *testing.T) {
		patches := []*commonmodels.JSONPatchOperation{
			{
				Operation: "replace",
				Path:      "/stages/0/jobs/0",
				Value: map[string]interface{}{
					"name":            "job-1",
					"type":            "",
					"spec":            nil,
					"run_policy":      "",
					"error_policy":    nil,
					"execute_policy":  nil,
					"service_modules": nil,
				},
			},
		}

		_, _, err := validateFrontendWorkflowDelta(base, patches)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "execute_policy must be consistent with workflow template")
	})

	t.Run("allows changes unrelated to execute policy", func(t *testing.T) {
		patches := []*commonmodels.JSONPatchOperation{
			{
				Operation: "replace",
				Path:      "/stages/0/jobs/0/run_policy",
				Value:     config.DefaultNotRun,
			},
		}

		validated, rendered, err := validateFrontendWorkflowDelta(base, patches)
		require.NoError(t, err)
		assert.Equal(t, patches, validated)
		assert.Equal(t, config.DefaultNotRun, rendered.Stages[0].Jobs[0].RunPolicy)
		assert.Equal(t, base.Stages[0].Jobs[0].ExecutePolicy, rendered.Stages[0].Jobs[0].ExecutePolicy)
	})
}

func TestUpdateWorkflowParamRuntimeTypes(t *testing.T) {
	params := []*commonmodels.Param{
		{Name: "reviewers", ParamsType: "multi-select", ChoiceOption: []string{"alice", "bob"}},
		{Name: "config", ParamsType: "file"},
	}
	err := UpdateProjectWorkflowParam(params, []*CreateCustomTaskParam{
		{Name: "reviewers", ChoiceValue: []string{"alice", "bob"}},
		{Name: "config", FileID: "file-id", FileName: "config.yaml", FilePath: "/zadig/files"},
	}, "demo")
	require.NoError(t, err)
	assert.Equal(t, []string{"alice", "bob"}, params[0].ChoiceValue)
	assert.Equal(t, "file-id", params[1].FileID)
}

func TestInputUpdaterCodehostScope(t *testing.T) {
	job := &commonmodels.Job{JobType: config.JobFreestyle}
	workflow := &commonmodels.WorkflowV4{Project: "demo"}

	legacyUpdater, err := GetInputUpdater(job, map[string]interface{}{}, workflow)
	require.NoError(t, err)
	assert.False(t, legacyUpdater.(*FreestyleJobInput).useProjectCodehosts)

	projectUpdater, err := getInputUpdater(job, map[string]interface{}{}, workflow, true)
	require.NoError(t, err)
	assert.True(t, projectUpdater.(*FreestyleJobInput).useProjectCodehosts)
}

func TestValidateOpenAPIRepositoryRef(t *testing.T) {
	tests := []struct {
		name         string
		branch       string
		tag          string
		pr           int
		prs          []int
		enableCommit bool
		commitID     string
		wantErr      bool
	}{
		{name: "branch", branch: "main"},
		{name: "pull requests", branch: "main", prs: []int{1, 2}},
		{name: "single pull request without target branch", pr: 1, wantErr: true},
		{name: "single pull request with target branch", branch: "main", pr: 1},
		{name: "tag", tag: "v1.0.0"},
		{name: "commit", enableCommit: true, commitID: "abc123"},
		{name: "missing ref", wantErr: true},
		{name: "branch and tag", branch: "main", tag: "v1.0.0", wantErr: true},
		{name: "pull requests without target branch", prs: []int{1}, wantErr: true},
		{name: "commit without id", enableCommit: true, wantErr: true},
		{name: "commit id without enable", commitID: "abc123", wantErr: true},
		{name: "commit and branch", branch: "main", enableCommit: true, commitID: "abc123", wantErr: true},
		{name: "single and multiple pull requests", pr: 1, prs: []int{1}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOpenAPIRepositoryRef(tt.branch, tt.tag, tt.pr, tt.prs, tt.enableCommit, tt.commitID)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNormalizeOpenAPIRepositoryInput(t *testing.T) {
	input := &types.OpenAPIRepoInput{Branch: "main", PR: 1}
	require.NoError(t, normalizeOpenAPIRepositoryInput(input))
	assert.Zero(t, input.PR)
	assert.Equal(t, []int{1}, input.PRs)

	err := normalizeOpenAPIRepositoryInput(&types.OpenAPIRepoInput{Branch: "main", PR: 1, PRs: []int{2}})
	require.Error(t, err)
}

func TestUpdateOpenAPIRepositoryRef(t *testing.T) {
	repo := &types.Repository{Branch: "main", MergeBranches: []string{"feature"}, CommitID: "old", CheckoutRef: "old-ref"}
	updateOpenAPIRepositoryRef(repo, "", "v1.0.0", 0, nil, false, "")

	assert.Empty(t, repo.Branch)
	assert.Equal(t, "v1.0.0", repo.Tag)
	assert.Empty(t, repo.MergeBranches)
	assert.Empty(t, repo.CommitID)
	assert.Empty(t, repo.CheckoutRef)
}

func TestOpenAPIRepoInputToRepositoryRejectsUnconfiguredRepository(t *testing.T) {
	codehosts := map[string]*codehostmodels.CodeHost{
		"project-gitlab": {ID: 2, Alias: "project-gitlab", IntegrationLevel: setting.IntegrationLevelProject, Project: "demo"},
	}
	originalRepos := []*types.Repository{{CodehostID: 2, RepoNamespace: "team", RepoName: "api", Branch: "main"}}
	_, err := openAPIRepoInputToRepository(originalRepos, []*types.OpenAPIRepoInput{{
		CodeHostName: "project-gitlab", RepoNamespace: "team", RepoName: "other", Branch: "main",
	}}, codehosts, true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in job")
}

func TestOpenAPIRepositoryOverrideCompatibility(t *testing.T) {
	codehosts := map[string]*codehostmodels.CodeHost{"gitlab": {ID: 1}}
	for _, projectScoped := range []bool{false, true} {
		repo := &types.Repository{CodehostID: 1, RepoOwner: "team", RepoName: "api", Tag: "old-tag", MergeBranches: []string{"old-branch"}}
		result, err := openAPIRepoInputToRepository([]*types.Repository{repo}, []*types.OpenAPIRepoInput{
			{CodeHostName: "gitlab", RepoNamespace: "team", RepoName: "api", Branch: "main"},
		}, codehosts, projectScoped)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "main", result[0].Branch)
		if projectScoped {
			assert.Empty(t, result[0].Tag)
			assert.Empty(t, result[0].MergeBranches)
		} else {
			assert.Equal(t, "old-tag", result[0].Tag)
			assert.Equal(t, []string{"old-branch"}, result[0].MergeBranches)
		}
	}
	result, err := openAPIRepoInputToRepository(nil, []*types.OpenAPIRepoInput{{CodeHostName: "gitlab", RepoName: "unknown"}}, codehosts, false)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestSanitizeOpenAPIWorkflowPreset(t *testing.T) {
	value := map[string]interface{}{
		"is_credential": true,
		"value":         "secret",
		"choice_option": []interface{}{"secret"},
		"nested": []interface{}{map[string]interface{}{
			"codehost_id": float64(1), "repo_owner": "team", "repo_namespace": "", "password": "password", "token": "token",
			"sonar_token": "sonar-token", "client_secret": "client-secret", "ak": "access-key", "sk": "secret-key",
			"api_key": "api-key", "private_key": "private-key", "hook_address": "webhook",
		}, map[string]interface{}{"address": "https://example.com/webhook", "token": "webhook-token"}},
	}

	require.NoError(t, sanitizeOpenAPIWorkflowPreset(value, map[int]string{1: "gitlab"}, nil))

	assert.Equal(t, "", value["value"])
	assert.Empty(t, value["choice_option"])
	assert.Equal(t, true, value["has_value"])
	nested := value["nested"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "", nested["password"])
	assert.Equal(t, "", nested["token"])
	assert.Equal(t, "", nested["sonar_token"])
	assert.Equal(t, "", nested["client_secret"])
	assert.Equal(t, "", nested["ak"])
	assert.Equal(t, "", nested["sk"])
	assert.Equal(t, "", nested["api_key"])
	assert.Equal(t, "", nested["private_key"])
	assert.Equal(t, "", nested["hook_address"])
	assert.Equal(t, "gitlab", nested["codehost_name"])
	assert.Equal(t, "team", nested["repo_namespace"])
	webhook := value["nested"].([]interface{})[1].(map[string]interface{})
	assert.Equal(t, "", webhook["address"])
	assert.Equal(t, "", webhook["token"])
}

func TestOpenAPIWorkflowDynamicChoicesUseSubmittedParameters(t *testing.T) {
	workflow := &commonmodels.WorkflowV4{Params: []*commonmodels.Param{
		{Name: "env", ParamsType: "string", Value: "staging"},
		{Name: "version", ParamsType: "choice", Value: "staging-v1", ChoiceOption: []string{"staging-v1"},
			Script: `func versions(env string) []string { return []string{env + "-v1"} }`, CallFunction: "versions({{.workflow.params.env}})"},
	}}
	require.NoError(t, UpdateProjectWorkflowParam(workflow.Params, []*CreateCustomTaskParam{
		{Name: "version", Value: "production-v1"},
		{Name: "env", Value: "production"},
	}, ""))
	require.NoError(t, controller.CreateWorkflowController(workflow).RenderWorkflowDynamicParams(0, "", "", "", nil))
	require.NoError(t, validateOpenAPIWorkflowChoices(workflow.Params))
	assert.Equal(t, []string{"production-v1"}, workflow.Params[1].ChoiceOption)
	workflow.Params[1].Value = "staging-v1"
	require.Error(t, validateOpenAPIWorkflowChoices(workflow.Params))
}

func TestOpenAPIWorkflowChoices(t *testing.T) {
	for _, tt := range []struct {
		name    string
		param   *commonmodels.Param
		wantErr bool
	}{
		{"optional empty", &commonmodels.Param{ParamsType: "choice"}, false},
		{"valid multi select", &commonmodels.Param{ParamsType: "multi-select", ChoiceOption: []string{"a", "b"}, ChoiceValue: []string{"b"}}, false},
		{"invalid multi select", &commonmodels.Param{ParamsType: "multi-select", ChoiceOption: []string{"a"}, ChoiceValue: []string{"b"}}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOpenAPIWorkflowChoices([]*commonmodels.Param{tt.param})
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestUpdateOpenAPIWorkflowJobs(t *testing.T) {
	for _, tt := range []struct {
		name    string
		inputs  []*CreateCustomTaskJobInput
		wantErr string
	}{
		{"unknown", []*CreateCustomTaskJobInput{{JobName: "typo"}}, "job not found"},
		{"duplicate", []*CreateCustomTaskJobInput{{JobName: "run"}, {JobName: "run"}}, "duplicate"},
		{"null", []*CreateCustomTaskJobInput{nil}, "job_name is required"},
		{"definition controls type", []*CreateCustomTaskJobInput{{JobName: "run", JobType: config.JobZadigBuild, Parameters: map[string]interface{}{}}}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			workflow := &commonmodels.WorkflowV4{Stages: []*commonmodels.WorkflowStage{{Jobs: []*commonmodels.Job{
				{Name: "run", JobType: config.JobAI, Skipped: true},
				{Name: "skip", JobType: config.JobAI},
			}}}}
			err := updateOpenAPIWorkflowJobs(workflow, tt.inputs)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.False(t, workflow.Stages[0].Jobs[0].Skipped)
			assert.True(t, workflow.Stages[0].Jobs[1].Skipped)
		})
	}
}

func TestOpenAPIBuildRejectsUnknownTarget(t *testing.T) {
	job := &commonmodels.Job{Spec: &commonmodels.ZadigBuildJobSpec{ServiceAndBuildsOptions: []*commonmodels.ServiceAndBuild{
		{ServiceName: "api", ServiceModule: "api"},
	}}}
	input := &ZadigBuildJobInput{
		OpenAPIBasicInfo: &OpenAPIBasicInfo{useProjectCodehosts: true},
		ServiceList:      []*types.OpenAPIServiceBuildArgs{{ServiceName: "api", ServiceModule: "api"}, {ServiceName: "api", ServiceModule: "typo"}},
	}
	_, err := input.UpdateJobSpec(job)
	require.ErrorContains(t, err, "target not configured")
}

func TestSanitizeOpenAPIWorkflowPresetRejectsAmbiguousCodehost(t *testing.T) {
	value := map[string]interface{}{"codehost_id": float64(1)}
	err := sanitizeOpenAPIWorkflowPreset(value, map[int]string{1: "gitlab"}, map[string]struct{}{"gitlab": {}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple code hosts named gitlab")
}

func TestSanitizeOpenAPIWorkflowPresetCredentialHasValue(t *testing.T) {
	tests := []struct {
		name  string
		value map[string]interface{}
		want  bool
	}{
		{name: "string value", value: map[string]interface{}{"type": "string", "value": "secret"}, want: true},
		{name: "default value", value: map[string]interface{}{"type": "choice", "default": "secret"}, want: true},
		{name: "multi select", value: map[string]interface{}{"type": "multi-select", "choice_value": []interface{}{"secret"}}, want: true},
		{name: "file ID", value: map[string]interface{}{"type": "file", "file_id": "file-id"}, want: true},
		{name: "file path", value: map[string]interface{}{"type": "file", "file_path": "/zadig/file"}, want: true},
		{name: "empty file", value: map[string]interface{}{"type": "file"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.value["is_credential"] = true
			require.NoError(t, sanitizeOpenAPIWorkflowPreset(tt.value, nil, nil))
			assert.Equal(t, tt.want, tt.value["has_value"])
			assert.Empty(t, tt.value["value"])
			assert.Empty(t, tt.value["default"])
			assert.Empty(t, tt.value["file_id"])
			assert.Empty(t, tt.value["file_path"])
			assert.Empty(t, tt.value["choice_value"])
		})
	}
}

func TestSanitizeOpenAPIWorkflowPresetRejectsUnavailableCodehost(t *testing.T) {
	value := map[string]interface{}{"codehost_id": float64(2), "repo_name": "api"}
	err := sanitizeOpenAPIWorkflowPreset(value, map[int]string{1: "gitlab"}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "code host with ID 2 is not available in project")
}
