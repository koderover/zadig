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
	githubservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/github"
)

func TestGitCheckUsesExplicitTaskURL(t *testing.T) {
	taskURL := "https://zadig.example/v1/projects/detail/demo/scanner/detail/review/task/7?id=scan-id&scannerType=ai_review"
	check := &githubservice.GitCheck{
		TaskURL:     taskURL,
		AslanURL:    "https://zadig.example",
		ProductName: "demo",
		PipeName:    "zadig-scanning-scan-id",
		DisplayName: "review",
		PipeType:    config.WorkflowTypeV4,
		TaskID:      7,
	}

	require.Equal(t, taskURL, check.DetailsURL())

	check.TaskURL = ""
	require.Equal(t, "https://zadig.example/v1/projects/detail/demo/pipelines/custom/zadig-scanning-scan-id/7?display_name=review", check.DetailsURL())
}

func TestMergeApolloRetryState(t *testing.T) {
	currentSpec := &commonmodels.JobTaskApolloSpec{
		NamespaceList: []*commonmodels.JobTaskApolloNamespace{
			nil,
			{
				ApolloNamespace: commonmodels.ApolloNamespace{
					Action:    commonmodels.ApolloActionUpdate,
					AppID:     "app",
					Env:       "dev",
					ClusterID: "default",
					Namespace: "passed",
				},
				Status: string(config.StatusPassed),
			},
			{
				ApolloNamespace: commonmodels.ApolloNamespace{
					Action:    commonmodels.ApolloActionUpdate,
					AppID:     "app",
					Env:       "dev",
					ClusterID: "default",
					Namespace: "failed",
				},
				Status: string(config.StatusFailed),
			},
			{
				ApolloNamespace: commonmodels.ApolloNamespace{
					Action:    commonmodels.ApolloActionUpdate,
					AppID:     "app",
					Env:       "dev",
					ClusterID: "default",
					Namespace: "running",
				},
				Status: string(config.StatusRunning),
			},
			{
				ApolloNamespace: commonmodels.ApolloNamespace{
					Action:    commonmodels.ApolloActionCreate,
					AppID:     "app",
					Namespace: "feature.yaml",
					Type:      "yaml",
				},
				AppNamespaceCreated: true,
				Status:              string(config.StatusFailed),
			},
		},
	}
	retrySpec := &commonmodels.JobTaskApolloSpec{
		NamespaceList: []*commonmodels.JobTaskApolloNamespace{
			nil,
			{
				ApolloNamespace: commonmodels.ApolloNamespace{
					Action:    commonmodels.ApolloActionUpdate,
					AppID:     "app",
					Env:       "dev",
					ClusterID: "default",
					Namespace: "passed",
				},
				Status: string(config.StatusCreated),
				Error:  "stale error",
			},
			{
				ApolloNamespace: commonmodels.ApolloNamespace{
					Action:    commonmodels.ApolloActionUpdate,
					AppID:     "app",
					Env:       "dev",
					ClusterID: "default",
					Namespace: "failed",
				},
				Status: string(config.StatusCreated),
				Error:  "stale error",
			},
			{
				ApolloNamespace: commonmodels.ApolloNamespace{
					Action:    commonmodels.ApolloActionUpdate,
					AppID:     "app",
					Env:       "dev",
					ClusterID: "default",
					Namespace: "running",
				},
				Status: string(config.StatusCreated),
				Error:  "stale error",
			},
			{
				ApolloNamespace: commonmodels.ApolloNamespace{
					Action:    commonmodels.ApolloActionCreate,
					AppID:     "app",
					Namespace: "feature",
					Type:      "yaml",
				},
				Status: string(config.StatusCreated),
				Error:  "stale error",
			},
			{
				ApolloNamespace: commonmodels.ApolloNamespace{
					Action:    commonmodels.ApolloActionUpdate,
					AppID:     "app",
					Env:       "dev",
					ClusterID: "default",
					Namespace: "unmatched",
				},
				Status: string(config.StatusCreated),
			},
		},
	}

	mergeApolloRetryState(currentSpec, retrySpec)

	require.Equal(t, string(config.StatusPassed), retrySpec.NamespaceList[1].Status)
	require.Empty(t, retrySpec.NamespaceList[1].Error)
	require.Equal(t, string(config.StatusCreated), retrySpec.NamespaceList[2].Status)
	require.Empty(t, retrySpec.NamespaceList[2].Error)
	require.Equal(t, string(config.StatusCreated), retrySpec.NamespaceList[3].Status)
	require.Empty(t, retrySpec.NamespaceList[3].Error)
	require.True(t, retrySpec.NamespaceList[4].AppNamespaceCreated)
	require.Equal(t, "feature.yaml", retrySpec.NamespaceList[4].Namespace)
	require.Equal(t, string(config.StatusCreated), retrySpec.NamespaceList[4].Status)
	require.Empty(t, retrySpec.NamespaceList[4].Error)
	require.Equal(t, string(config.StatusCreated), retrySpec.NamespaceList[5].Status)
}

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
