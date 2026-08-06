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
