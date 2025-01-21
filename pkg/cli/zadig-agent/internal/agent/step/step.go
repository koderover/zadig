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

package step

import (
	"context"
	"errors"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/archive"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/docker"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/git"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/perforce"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/scanning"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/script"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/testing"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	jobctl "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller/jobcontroller"
)

type StepInfos struct {
	Step      *types.Step
	Workspace string
	Paths     string
	Envs      []string
}

type Step interface {
	Run(ctx context.Context) error
}

func RunStep(ctx context.Context, jobCtx *jobctl.JobContext, step *commonmodels.StepTask, dirs *types.AgentWorkDirs, envs, secretEnvs []string, logger *log.JobLogger) error {
	var stepInstance Step
	var err error

	if step == nil {
		log.Errorf("step is nil")
		return errors.New("step is nil")
	}

	switch step.StepType {
	case "batch_file":
		stepInstance, err = script.NewBatchFileStep(jobCtx.Outputs, step.Spec, dirs, envs, secretEnvs, logger)
		if err != nil {
			return err
		}
	case "powershell":
		stepInstance, err = script.NewPowerShellStep(jobCtx.Outputs, step.Spec, dirs, envs, secretEnvs, logger)
		if err != nil {
			return err
		}
	case "shell":
		stepInstance, err = script.NewShellStep(jobCtx.Outputs, step.Spec, dirs, envs, secretEnvs, logger)
		if err != nil {
			return err
		}
	case "git":
		stepInstance, err = git.NewGitStep(step.Spec, dirs, envs, secretEnvs, logger)
		if err != nil {
			return err
		}
	case "perforce":
		stepInstance, err = perforce.NewP4Step(step.Spec, dirs, envs, secretEnvs, logger)
		if err != nil {
			return err
		}
	case "docker_build":
		stepInstance, err = docker.NewDockerBuildStep(step.Spec, dirs, envs, secretEnvs, logger)
		if err != nil {
			return err
		}
	case "archive":
		stepInstance, err = archive.NewArchiveStep(step.Spec, dirs, envs, secretEnvs, logger)
		if err != nil {
			return err
		}
	case "archive_html":
		stepInstance, err = archive.NewArchiveHtmlStep(step.Spec, dirs, envs, secretEnvs, logger)
		if err != nil {
			return err
		}
	case "tar_archive":
		stepInstance, err = archive.NewTarArchiveStep(step.Spec, dirs, envs, secretEnvs, logger)
		if err != nil {
			return err
		}
	case "download_archive":
		stepInstance, err = archive.NewDownloadArchiveStep(step.Spec, dirs, envs, secretEnvs, logger)
		if err != nil {
			return err
		}
	case "junit_report":
		stepInstance, err = testing.NewJunitReportStep(step.Spec, dirs, envs, secretEnvs, logger)
		if err != nil {
			return err
		}
	case "sonar_check":
		stepInstance, err = scanning.NewSonarCheckStep(step.Spec, dirs, envs, secretEnvs, logger)
		if err != nil {
			return err
		}
	case "sonar_get_metrics":
		stepInstance, err = scanning.NewSonarGetMetricsStep(step.Spec, dirs, envs, secretEnvs, logger)
		if err != nil {
			return err
		}
	case "tools":
		return nil
	case "debug_before":
		return nil
	case "debug_after":
		return nil
	default:
		err := fmt.Errorf("step type: %s does not match any known type", step.StepType)
		log.Error(err)
		return err
	}
	if err := stepInstance.Run(ctx); err != nil {
		return err
	}
	return nil
}
