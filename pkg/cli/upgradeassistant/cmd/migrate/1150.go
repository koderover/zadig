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
	upgradepath.RegisterHandler("1.15.0", "1.14.0", V1150ToV1140)
}

func V1140ToV1150() error {
	// refactor workflow_task_v4
	if err := workflowV4JobRefactor(); err != nil {
		log.Errorf("workflowV4JobRefactor err:%s", err)
		return err
	}
	if err := deleteXenialBasicImage(); err != nil {
		log.Errorf("delete xenial basic image err:%s", err)
		return err
	}
	return nil
}

func V1150ToV1140() error {
	// roll back workflow_task_v4
	if err := workflowV4JobRollback(); err != nil {
		log.Errorf("workflowV4JobRollback err:%s", err)
		return err
	}
	return nil
}

func deleteXenialBasicImage() error {
	xenialBasicImage, focalBasicImage, err := internalmongodb.NewBasicImageColl().FindXenialAndFocalBasicImage()
	if err != nil {
		return nil
	}
	buildCollection := internalmongodb.NewBuildColl()
	buildList, err := buildCollection.List(&internalmongodb.BuildListOption{})
	if err != nil {
		return err
	}
	var ms []mongo.WriteModel
	for _, build := range buildList {
		if build.PreBuild.BuildOS == "xenial" {
			build.PreBuild.BuildOS = "focal"
			build.PreBuild.ImageID = focalBasicImage.ID.Hex()
			ms = append(ms, mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", build.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"pre_build", build.PreBuild},
					}},
				}),
			)
		}
	}
	if len(ms) > 0 {
		_, err := buildCollection.BulkWrite(context.TODO(), ms)
		if err != nil {
			return err
		}
	}

	testingCollection := internalmongodb.NewTestingColl()
	testList, err := testingCollection.List(&internalmongodb.ListTestOption{})
	if err != nil {
		return err
	}
	ms = make([]mongo.WriteModel, 0)
	for _, test := range testList {
		if test.PreTest.BuildOS == "xenial" {
			test.PreTest.BuildOS = "focal"
			test.PreTest.ImageID = focalBasicImage.ID.Hex()
			ms = append(ms, mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", test.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"pre_test", test.PreTest},
					}},
				}),
			)
		}
	}

	if len(ms) > 0 {
		_, err := testingCollection.BulkWrite(context.TODO(), ms)
		if err != nil {
			return err
		}
	}

	scanningColl := internalmongodb.NewScanningColl()
	scanningList, err := scanningColl.List(&internalmongodb.ScanningListOption{})
	if err != nil {
		return err
	}
	ms = make([]mongo.WriteModel, 0)
	for _, scanning := range scanningList {
		if scanning.ImageID == xenialBasicImage.ID.Hex() {
			ms = append(ms, mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", scanning.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"image_id", focalBasicImage.ID.Hex()},
					}},
				}),
			)
		}
	}

	if len(ms) > 0 {
		_, err := scanningColl.BulkWrite(context.TODO(), ms)
		if err != nil {
			return err
		}
	}

	err = internalmongodb.NewBasicImageColl().RemoveXenial()
	if err != nil {
		log.Warnf("delete xenial basic image failed, err: %s", err)
	}
	return nil
}

func workflowV4JobRefactor() error {
	workflowtaskCol, err := internalmongodb.NewWorkflowTaskV4Coll().List()
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

func workflowV4JobRollback() error {
	workflowtaskCol, err := internalmongodb.NewWorkflowTaskV4Coll().List()
	if err != nil {
		return fmt.Errorf("failed to list all data in `zadig.workflow_task_v4`,err: %s", err)
	}

	if len(workflowtaskCol) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, workflowTask := range workflowtaskCol {
		rollBackStageTask, err := rollBackStagetask(workflowTask.Stages)
		if err != nil {
			log.Error(err)
			continue
		}
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflowTask.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"stages", rollBackStageTask},
					}},
				}),
		)
	}
	if len(ms) > 0 {
		_, err = internalmongodb.NewWorkflowTaskV4Coll().BulkWrite(context.TODO(), ms)
	}

	return err
}

func transformStagetask(stageTask []*models.StageTask) ([]*models.StageTask, error) {
	for _, originStage := range stageTask {
		for _, originJob := range originStage.Jobs {
			if originJob.Spec != nil {
				return stageTask, fmt.Errorf("stage already upgraded")
			}
			switch originJob.JobType {
			case string(config.JobZadigBuild):
				jobSpec := &commonmodels.JobTaskFreestyleSpec{
					Properties: originJob.Properties,
					Steps:      originJob.Steps,
				}
				originJob.Spec = jobSpec
			case string(config.JobZadigDeploy):
				for _, stepTask := range originJob.Steps {
					if stepTask.StepType == config.StepHelmDeploy {
						stepSpec := &models.StepHelmDeploySpec{}
						if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil {
							return stageTask, fmt.Errorf("unmashal step spec error: %v", err)
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
						originJob.Spec = jobSpec
						originJob.JobType = string(config.JobZadigHelmDeploy)
						break
					}
					if stepTask.StepType == config.StepDeploy {
						stepSpec := &models.StepDeploySpec{}
						if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil {
							return stageTask, fmt.Errorf("unmashal step spec error: %v", err)
						}
						jobSpec := &commonmodels.JobTaskDeploySpec{
							Env:                stepSpec.Env,
							ServiceName:        stepSpec.ServiceName,
							ServiceModule:      stepSpec.ServiceModule,
							ServiceType:        setting.K8SDeployType,
							SkipCheckRunStatus: stepSpec.SkipCheckRunStatus,
							Image:              stepSpec.Image,
							ClusterID:          stepSpec.ClusterID,
							Timeout:            stepSpec.Timeout,
							ReplaceResources:   stepSpec.ReplaceResources,
						}
						originJob.Spec = jobSpec
						break
					}
				}
			case string(config.JobPlugin):
				jobSpec := &commonmodels.JobTaskPluginSpec{
					Properties: originJob.Properties,
					Plugin:     originJob.Plugin,
				}
				originJob.Spec = jobSpec
			case string(config.JobFreestyle):
				jobSpec := &commonmodels.JobTaskFreestyleSpec{
					Properties: originJob.Properties,
					Steps:      originJob.Steps,
				}
				originJob.Spec = jobSpec
			case string(config.JobCustomDeploy):
				for _, stepTask := range originJob.Steps {
					if stepTask.StepType == config.StepCustomDeploy {
						stepSpec := &models.StepCustomDeploySpec{}
						if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil {
							return stageTask, fmt.Errorf("unmashal step spec error: %v", err)
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
						originJob.Spec = jobSpec
						break
					}
				}
			}
		}
	}
	return stageTask, nil
}

func rollBackStagetask(stageTask []*models.StageTask) ([]*models.StageTask, error) {
	for _, originStage := range stageTask {
		for _, originJob := range originStage.Jobs {
			if len(originJob.Steps) != 0 || originJob.Plugin != nil {
				return stageTask, fmt.Errorf("stage do not need to rollback")
			}
			switch originJob.JobType {
			case string(config.JobZadigBuild):
				jobSpec := &commonmodels.JobTaskFreestyleSpec{}
				if err := commonmodels.IToi(originJob.Spec, jobSpec); err != nil {
					return stageTask, fmt.Errorf("unmashal job spec error: %v", err)
				}
				originJob.Steps = jobSpec.Steps
				originJob.Properties = jobSpec.Properties

			case string(config.JobZadigDeploy):
				stepTask := &commonmodels.StepTask{
					Name:     originJob.Name,
					StepType: config.StepDeploy,
				}
				jobSpec := &commonmodels.JobTaskDeploySpec{}
				if err := commonmodels.IToi(originJob.Spec, jobSpec); err != nil {
					return stageTask, fmt.Errorf("unmashal job spec error: %v", err)
				}
				stepSpec := &models.StepDeploySpec{
					Env:                jobSpec.Env,
					ServiceName:        jobSpec.ServiceName,
					ServiceType:        setting.K8SDeployType,
					ServiceModule:      jobSpec.ServiceModule,
					SkipCheckRunStatus: jobSpec.SkipCheckRunStatus,
					ClusterID:          jobSpec.ClusterID,
					Timeout:            jobSpec.Timeout,
					Image:              jobSpec.Image,
					ReplaceResources:   jobSpec.ReplaceResources,
				}
				stepTask.Spec = stepSpec
				originJob.Steps = []*commonmodels.StepTask{stepTask}

			case string(config.JobZadigHelmDeploy):
				originJob.JobType = string(config.JobZadigDeploy)
				stepTask := &commonmodels.StepTask{
					Name:     originJob.Name,
					StepType: config.StepHelmDeploy,
				}
				jobSpec := &commonmodels.JobTaskHelmDeploySpec{}
				if err := commonmodels.IToi(originJob.Spec, jobSpec); err != nil {
					return stageTask, fmt.Errorf("unmashal job spec error: %v", err)
				}
				stepSpec := &models.StepHelmDeploySpec{
					Env:                jobSpec.Env,
					ServiceName:        jobSpec.ServiceName,
					ServiceType:        setting.HelmDeployType,
					SkipCheckRunStatus: jobSpec.SkipCheckRunStatus,
					ImageAndModules:    jobSpec.ImageAndModules,
					ClusterID:          jobSpec.ClusterID,
					ReleaseName:        jobSpec.ReleaseName,
					Timeout:            jobSpec.Timeout,
					ReplaceResources:   jobSpec.ReplaceResources,
				}
				stepTask.Spec = stepSpec
				originJob.Steps = []*commonmodels.StepTask{stepTask}

			case string(config.JobPlugin):
				jobSpec := &commonmodels.JobTaskPluginSpec{}
				if err := commonmodels.IToi(originJob.Spec, jobSpec); err != nil {
					return stageTask, fmt.Errorf("unmashal job spec error: %v", err)
				}
				originJob.Properties = jobSpec.Properties
				originJob.Plugin = jobSpec.Plugin

			case string(config.JobFreestyle):
				jobSpec := &commonmodels.JobTaskFreestyleSpec{}
				if err := commonmodels.IToi(originJob.Spec, jobSpec); err != nil {
					return stageTask, fmt.Errorf("unmashal job spec error: %v", err)
				}
				originJob.Steps = jobSpec.Steps
				originJob.Properties = jobSpec.Properties

			case string(config.JobCustomDeploy):
				stepTask := &commonmodels.StepTask{
					Name:     originJob.Name,
					StepType: config.StepCustomDeploy,
				}
				jobSpec := &commonmodels.JobTaskCustomDeploySpec{}
				if err := commonmodels.IToi(originJob.Spec, jobSpec); err != nil {
					return stageTask, fmt.Errorf("unmashal job spec error: %v", err)
				}
				stepSpec := &models.StepCustomDeploySpec{
					Namespace:          jobSpec.Namespace,
					ClusterID:          jobSpec.ClusterID,
					Timeout:            jobSpec.Timeout,
					WorkloadType:       jobSpec.WorkloadType,
					WorkloadName:       jobSpec.WorkloadName,
					ContainerName:      jobSpec.ContainerName,
					SkipCheckRunStatus: jobSpec.SkipCheckRunStatus,
					Image:              jobSpec.Image,
					ReplaceResources:   jobSpec.ReplaceResources,
				}
				stepTask.Spec = stepSpec
				originJob.Steps = []*commonmodels.StepTask{stepTask}
			}
		}
	}
	return stageTask, nil
}
