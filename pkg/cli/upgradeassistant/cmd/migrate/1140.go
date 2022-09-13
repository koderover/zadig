/*
Copyright 2022 The KodeRover Authors.

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

package migrate

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	upgradepath.RegisterHandler("1.14.0", "1.15.0", V1140ToV1150)
	upgradepath.RegisterHandler("1.15.0", "1.14.0", V1120ToV1110)
}

func V1140ToV1150() error {
	// refactor workflow_task_v4
	if err := workflowV4JobRefactor(); err != nil {
		log.Errorf("workflowV4JobRefactor err:%s", err)
		return err
	}
	return nil
}

func V1150ToV1140() error {
	return nil
}

func workflowV4JobRefactor() error {
	workflowtaskCol, err := internalmongodb.NewWorkflowTaskV4Coll().ListAll()
	if err != nil {
		return fmt.Errorf("failed to list all data in `zadig.workflow_task_v4`,err: %s", err)
	}

	if len(workflowtaskCol) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, workflowTask := range workflowtaskCol {
		newStageTask, err := transformStagetask(workflowTask.Stages)
		if err != nil {
			log.Error(err)
			continue
		}
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflowTask.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"stages", newStageTask},
					}},
				}),
		)
	}
	if len(ms) > 0 {
		_, err = internalmongodb.NewWorkflowTaskV4Coll().BulkWrite(context.TODO(), ms)
	}

	return err
}

func transformStagetask(stageTask []*models.StageTask) ([]*commonmodels.StageTask, error) {
	resp := []*commonmodels.StageTask{}
	for _, originStage := range stageTask {
		newJobs := []*commonmodels.JobTask{}
		for _, originJob := range originStage.Jobs {
			if len(originJob.Steps) == 0 && originJob.Plugin == nil {
				return resp, fmt.Errorf("not a illegal job input")
			}
			newJob := &commonmodels.JobTask{
				Name:      originJob.Name,
				JobType:   originJob.JobType,
				Status:    originJob.Status,
				StartTime: originJob.StartTime,
				EndTime:   originJob.EndTime,
				Error:     originJob.Error,
				Timeout:   originJob.EndTime,
				Outputs:   originJob.Outputs,
			}
			switch originJob.JobType {
			case string(config.JobZadigBuild):
				jobSpec := &commonmodels.JobTaskBuildSpec{
					Properties: originJob.Properties,
					Steps:      originJob.Steps,
				}
				newJob.Spec = jobSpec
			case string(config.JobZadigDeploy):
				for _, stepTask := range originJob.Steps {
					if stepTask.StepType == config.StepHelmDeploy {
						stepSpec := &models.StepHelmDeploySpec{}
						if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil {
							return resp, fmt.Errorf("unmashal step spec error: %v", err)
						}
						jobSpec := &commonmodels.JobTaskHelmDeploySpec{
							Env:                stepSpec.Env,
							ServiceName:        stepSpec.ServiceName,
							ServiceType:        setting.HelmDeployType,
							SkipCheckRunStatus: stepSpec.SkipCheckRunStatus,
							ImageAndModules:    stepSpec.ImageAndModules,
							ClusterID:          stepSpec.ClusterID,
							ReleaseName:        stepSpec.ReleaseName,
							Timeout:            stepSpec.Timeout,
							ReplaceResources:   stepSpec.ReplaceResources,
						}
						newJob.Spec = jobSpec
						newJob.JobType = string(config.JobZadigHelmDeploy)
						break
					}
					if stepTask.StepType == config.StepDeploy {
						stepSpec := &models.StepDeploySpec{}
						if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil {
							return resp, fmt.Errorf("unmashal step spec error: %v", err)
						}
						jobSpec := &commonmodels.JobTaskDeploySpec{
							Env:                stepSpec.Env,
							ServiceName:        stepSpec.ServiceName,
							ServiceType:        setting.K8SDeployType,
							SkipCheckRunStatus: stepSpec.SkipCheckRunStatus,
							Image:              stepSpec.Image,
							ClusterID:          stepSpec.ClusterID,
							Timeout:            stepSpec.Timeout,
							ReplaceResources:   stepSpec.ReplaceResources,
						}
						newJob.Spec = jobSpec
						break
					}
				}
			case string(config.JobPlugin):
				jobSpec := &commonmodels.JobTaskPluginSpec{
					Properties: originJob.Properties,
					Plugin:     originJob.Plugin,
				}
				newJob.Spec = jobSpec
			case string(config.JobFreestyle):
				jobSpec := &commonmodels.JobTaskBuildSpec{
					Properties: originJob.Properties,
					Steps:      originJob.Steps,
				}
				newJob.Spec = jobSpec
			case string(config.JobCustomDeploy):
				for _, stepTask := range originJob.Steps {
					if stepTask.StepType == config.StepCustomDeploy {
						stepSpec := &models.StepCustomDeploySpec{}
						if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil {
							return resp, fmt.Errorf("unmashal step spec error: %v", err)
						}
						jobSpec := &commonmodels.JobTaskCustomDeploySpec{
							Namespace:          stepSpec.Namespace,
							ClusterID:          stepSpec.ClusterID,
							Timeout:            stepSpec.Timeout,
							WorkloadType:       stepSpec.WorkloadType,
							WorkloadName:       stepSpec.WorkloadName,
							ContainerName:      stepSpec.ContainerName,
							Image:              stepSpec.Image,
							SkipCheckRunStatus: stepSpec.SkipCheckRunStatus,
							ReplaceResources:   stepSpec.ReplaceResources,
						}
						newJob.Spec = jobSpec
						break
					}
				}
			}
			newJobs = append(newJobs, newJob)
		}
		newStage := commonmodels.StageTask{
			Name:      originStage.Name,
			Status:    originStage.Status,
			StartTime: originStage.StartTime,
			EndTime:   originStage.EndTime,
			Parallel:  originStage.Parallel,
			Approval:  originStage.Approval,
			Error:     originStage.Error,
			Jobs:      newJobs,
		}
		resp = append(resp, &newStage)
	}
	return resp, nil
}
