/*
Copyright 2026 The KodeRover Authors.

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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
)

type workflowEnvironmentPermission struct {
	allEnvironments           bool
	allProductionEnvironments bool
	readableEnvironments      sets.String
}

func getWorkflowEnvironmentPermission(userID string, authorizedResources *user.AuthorizedResources, workflow *commonmodels.WorkflowV4) (*workflowEnvironmentPermission, error) {
	if !hasEnvironmentJob(workflow) {
		return nil, nil
	}

	// authorizedResources == nil means the caller has not resolved the user's authorization info
	// yet. When there is no user id either, this is an in-process system trigger (webhook, workflow
	// trigger, release plan, ...), where the environment is fixed by configuration rather than
	// selected by a user, so there is nothing to restrict. An empty user id is NOT granted admin
	// here: callers that already resolved authorization (e.g. NewContextWithAuthorization) pass it
	// in, and empty-user-id requests are handled by their internal-token validation.
	if authorizedResources == nil {
		if userID == "" {
			return nil, nil
		}
		var err error
		authorizedResources, err = user.New().GetUserAuthInfo(userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user authorization info: %w", err)
		}
		if authorizedResources == nil {
			return nil, fmt.Errorf("empty user authorization info")
		}
	}

	permission := &workflowEnvironmentPermission{readableEnvironments: sets.NewString()}
	if authorizedResources.IsSystemAdmin {
		permission.allEnvironments = true
		permission.allProductionEnvironments = true
		return permission, nil
	}

	if projectActions := authorizedResources.ProjectAuthInfo[workflow.Project]; projectActions != nil {
		if projectActions.IsProjectAdmin {
			permission.allEnvironments = true
			permission.allProductionEnvironments = true
			return permission, nil
		}
		permission.allEnvironments = projectActions.Env != nil && projectActions.Env.View
		permission.allProductionEnvironments = projectActions.ProductionEnv != nil && projectActions.ProductionEnv.View
		if permission.allEnvironments && permission.allProductionEnvironments {
			return permission, nil
		}
	}

	collaborationPermission, err := user.New().ListCollaborationEnvironmentsPermission(userID, workflow.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to get collaboration environment permission: %w", err)
	}
	if collaborationPermission != nil {
		permission.readableEnvironments.Insert(collaborationPermission.ReadEnvList...)
	}
	return permission, nil
}

func (permission *workflowEnvironmentPermission) isAllowed(environment string, production bool) bool {
	if permission == nil || (production && permission.allProductionEnvironments) || (!production && permission.allEnvironments) {
		return true
	}
	return permission.readableEnvironments.Has(environment)
}

func filterWorkflowEnvironmentOptions(workflow *commonmodels.WorkflowV4, permission *workflowEnvironmentPermission) error {
	if permission == nil {
		return nil
	}
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if !isEnvironmentJob(job.JobType) {
				continue
			}
			spec, err := workflowJobSpec(job)
			if err != nil {
				return err
			}
			production, _ := spec["production"].(bool)
			filteredOptions := make([]interface{}, 0)
			if options, ok := spec["env_options"].([]interface{}); ok {
				for _, rawOption := range options {
					option, ok := rawOption.(map[string]interface{})
					if ok && permission.isAllowed(stringValue(option["env"]), production) {
						filteredOptions = append(filteredOptions, option)
					}
				}
			}
			spec["env_options"] = filteredOptions

			environment := environmentFromJobSpec(job.JobType, spec)
			if environment != "" && !permission.isAllowed(environment, production) {
				clearWorkflowJobEnvironment(job.JobType, spec)
			}
			job.Spec = spec
		}
	}
	return nil
}

func validateWorkflowEnvironmentSelection(workflow *commonmodels.WorkflowV4, permission *workflowEnvironmentPermission) error {
	if permission == nil {
		return nil
	}
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.Skipped || !isEnvironmentJob(job.JobType) {
				continue
			}
			spec, err := workflowJobSpec(job)
			if err != nil {
				return err
			}
			production, _ := spec["production"].(bool)
			environment := environmentFromJobSpec(job.JobType, spec)
			if environment != "" && !permission.isAllowed(environment, production) {
				return fmt.Errorf("workflow task creation denied: job %s cannot use environment %s without environment permission", job.Name, environment)
			}
		}
	}
	return nil
}

func hasEnvironmentJob(workflow *commonmodels.WorkflowV4) bool {
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if isEnvironmentJob(job.JobType) {
				return true
			}
		}
	}
	return false
}

func isEnvironmentJob(jobType config.JobType) bool {
	switch jobType {
	case config.JobZadigDeploy, config.JobZadigRestart, config.JobZadigHelmChartDeploy,
		config.JobK8sBlueGreenDeploy, config.JobZadigVMDeploy, config.JobSAEDeploy:
		return true
	default:
		return false
	}
}

func workflowJobSpec(job *commonmodels.Job) (map[string]interface{}, error) {
	spec := make(map[string]interface{})
	if err := commonmodels.IToi(job.Spec, &spec); err != nil {
		return nil, fmt.Errorf("failed to decode job %s spec: %w", job.Name, err)
	}
	return spec, nil
}

func environmentFromJobSpec(jobType config.JobType, spec map[string]interface{}) string {
	if jobType == config.JobSAEDeploy {
		envConfig, _ := spec["env_config"].(map[string]interface{})
		return stringValue(envConfig["name"])
	}
	environment := stringValue(spec["env"])
	if jobType == config.JobZadigVMDeploy {
		environment = strings.ReplaceAll(environment, setting.FixedValueMark, "")
	}
	return environment
}

func clearWorkflowJobEnvironment(jobType config.JobType, spec map[string]interface{}) {
	if jobType == config.JobSAEDeploy {
		if envConfig, ok := spec["env_config"].(map[string]interface{}); ok {
			envConfig["name"] = ""
		}
		return
	}
	spec["env"] = ""
}

func stringValue(value interface{}) string {
	valueString, _ := value.(string)
	return valueString
}
