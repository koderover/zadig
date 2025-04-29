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

package migrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	steptype "github.com/koderover/zadig/v2/pkg/types/step"
)

func init() {
	// 3.2.0 to 3.3.0
	upgradepath.RegisterHandler("3.3.0", "3.4.0", V330ToV340)
	upgradepath.RegisterHandler("3.4.0", "3.3.0", V340ToV330)
}

func V330ToV340() error {
	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}

	if !migrationInfo.UpdateWorkflow340JobSpec {
		workflowCursor, err := commonrepo.NewWorkflowV4Coll().ListByCursor(&commonrepo.ListWorkflowV4Option{})
		if err != nil {
			return fmt.Errorf("failed to list all custom workflow to update, error: %s", err)
		}
		for workflowCursor.Next(context.Background()) {
			workflow := new(commonmodels.WorkflowV4)
			if err := workflowCursor.Decode(workflow); err != nil {
				// continue converting to have maximum converage
				log.Warnf(err.Error())
			}

			log.Infof("migrating workflow: %s in project: %s ......", workflow.Name, workflow.Project)

			err = updateWorkflowJobTaskSpec(workflow.Stages)
			if err != nil {
				// continue converting to have maximum converage
				log.Warnf(err.Error())
			}

			err = commonrepo.NewWorkflowV4Coll().Update(
				workflow.ID.Hex(),
				workflow,
			)
			if err != nil {
				log.Warnf("failed to update workflow: %s in project %s, error: %s", workflow.Name, workflow.Project, err)
			}
			log.Infof("workflow: %s migration done ......", workflow.Name)
		}

		_ = mongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
			"update_workflow_340_job_spec": true,
		})
	}

	if !migrationInfo.UpdateWorkflow340JobTemplateSpec {
		templateCursor, err := commonrepo.NewWorkflowV4TemplateColl().ListByCursor(&commonrepo.ListWorkflowV4TemplateOption{})
		for templateCursor.Next((context.Background())) {
			workflowTemplate := new(commonmodels.WorkflowV4Template)
			if err := templateCursor.Decode(workflowTemplate); err != nil {
				// continue converting to have maximum converage
				log.Warnf(err.Error())
			}

			err = updateWorkflowJobTaskSpec(workflowTemplate.Stages)
			if err != nil {
				// continue converting to have maximum converage
				log.Warnf(err.Error())
			}

			err = commonrepo.NewWorkflowV4TemplateColl().Update(
				workflowTemplate,
			)
			if err != nil {
				log.Warnf("failed to update workflow template: %s, error: %s", workflowTemplate.TemplateName, err)
			}
		}

		_ = mongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
			"update_workflow_340_job_template_spec": true,
		})
	}

	return nil
}

func updateWorkflowJobTaskSpec(stages []*commonmodels.WorkflowStage) error {
	for _, stage := range stages {
		for _, job := range stage.Jobs {
			switch job.JobType {
			case config.JobApollo:
				newSpec := new(commonmodels.ApolloJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode apollo job, error: %s", err)
				}
				newSpec.NamespaceListOption = newSpec.NamespaceList
				newSpec.NamespaceList = make([]*commonmodels.ApolloNamespace, 0)
				job.Spec = newSpec
			case config.JobK8sBlueGreenDeploy:
				newSpec := new(commonmodels.BlueGreenDeployV2JobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				newSpec.ServiceOptions = newSpec.Services
				newSpec.Services = make([]*commonmodels.BlueGreenDeployV2Service, 0)
				job.Spec = newSpec
			case config.JobBuild:
				newSpec := new(commonmodels.ZadigBuildJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				newSpec.ServiceAndBuildsOptions = newSpec.ServiceAndBuilds
				newSpec.ServiceAndBuilds = make([]*commonmodels.ServiceAndBuild, 0)
				for _, svcBuildConfig := range newSpec.ServiceAndBuildsOptions {
					updateKeyVal(svcBuildConfig.KeyVals)
				}
				job.Spec = newSpec
			case config.JobK8sCanaryDeploy:
				newSpec := new(commonmodels.CanaryDeployJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				newSpec.TargetOptions = newSpec.Targets
				newSpec.Targets = make([]*commonmodels.CanaryTarget, 0)
				job.Spec = newSpec
			case config.JobCustomDeploy:
				newSpec := new(commonmodels.CustomDeployJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				newSpec.TargetOptions = newSpec.Targets
				newSpec.Targets = make([]*commonmodels.DeployTargets, 0)
				job.Spec = newSpec
			case config.JobZadigDeploy:
				newSpec := new(commonmodels.ZadigDeployJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				if strings.HasPrefix(newSpec.Env, "<+fixed>") {
					newSpec.Env = strings.TrimPrefix(newSpec.Env, "<+fixed>")
					newSpec.EnvSource = config.ParamSourceFixed
				} else {
					newSpec.EnvSource = config.ParamSourceRuntime
				}
				job.Spec = newSpec
			case config.JobFreestyle:
				// TODO: Add freestyle job ua logic
				newSpec := new(commonmodels.FreestyleJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				convertedSpec, err := converOldFreestyleJobSpec(newSpec)
				if err != nil {
					return err
				}
				job.Spec = convertedSpec
			case config.JobGrafana:
				newSpec := new(commonmodels.GrafanaJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				newSpec.AlertOptions = newSpec.Alerts
				newSpec.Alerts = make([]*commonmodels.GrafanaAlert, 0)
				job.Spec = newSpec
			case config.JobK8sGrayRelease:
				newSpec := new(commonmodels.GrayReleaseJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				newSpec.TargetOptions = newSpec.Targets
				newSpec.Targets = make([]*commonmodels.GrayReleaseTarget, 0)
				job.Spec = newSpec
			case config.JobJenkins:
				newSpec := new(commonmodels.JenkinsJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				newSpec.JobOptions = newSpec.Jobs
				newSpec.Jobs = make([]*commonmodels.JenkinsJobInfo, 0)
				job.Spec = newSpec
			case config.JobK8sPatch:
				newSpec := new(commonmodels.K8sPatchJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				newSpec.PatchItemOptions = newSpec.PatchItems
				newSpec.PatchItems = make([]*commonmodels.PatchItem, 0)
				job.Spec = newSpec
			case config.JobNacos:
				newSpec := new(commonmodels.NacosJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				newDefaultNacos := make([]*types.NacosDataID, 0)
				for _, nacosData := range newSpec.NacosDatas {
					newDefaultNacos = append(newDefaultNacos, &types.NacosDataID{
						DataID: nacosData.DataID,
						Group:  nacosData.Group,
					})
				}
				newSpec.DefaultNacosDatas = newDefaultNacos
				newSpec.NacosDatas = make([]*types.NacosConfig, 0)
				if strings.HasPrefix(newSpec.NamespaceID, "<+fixed>") {
					newSpec.NamespaceID = strings.TrimPrefix(newSpec.NamespaceID, "<+fixed>")
					newSpec.Source = config.ParamSourceFixed
				} else {
					newSpec.Source = config.ParamSourceRuntime
				}
				job.Spec = newSpec
			case config.JobPlugin:
				newSpec := new(commonmodels.PluginJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				advancedSetting := &commonmodels.JobAdvancedSettings{
					Timeout:             newSpec.Properties.Timeout,
					ClusterID:           newSpec.Properties.ClusterID,
					ClusterSource:       newSpec.Properties.ClusterSource,
					ResourceRequest:     newSpec.Properties.ResourceRequest,
					ResReqSpec:          newSpec.Properties.ResReqSpec,
					StrategyID:          newSpec.Properties.StrategyID,
					UseHostDockerDaemon: newSpec.Properties.UseHostDockerDaemon,
					CustomAnnotations:   newSpec.Properties.CustomAnnotations,
					CustomLabels:        newSpec.Properties.CustomLabels,
					ShareStorageInfo:    newSpec.Properties.ShareStorageInfo,
				}
				newSpec.AdvancedSetting = advancedSetting
				job.Spec = newSpec
			case config.JobZadigScanning:
				newSpec := new(commonmodels.ZadigScanningJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				newSpec.ScanningOptions = newSpec.Scannings
				newSpec.Scannings = make([]*commonmodels.ScanningModule, 0)
				newSpec.ServiceScanningOptions = newSpec.ServiceAndScannings
				newSpec.ServiceAndScannings = make([]*commonmodels.ServiceAndScannings, 0)

				for _, scanning := range newSpec.ScanningOptions {
					updateKeyVal(scanning.KeyVals)
				}

				for _, svc := range newSpec.ServiceScanningOptions {
					updateKeyVal(svc.KeyVals)
				}
				job.Spec = newSpec
			case config.JobZadigTesting:
				newSpec := new(commonmodels.ZadigTestingJobSpec)
				if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
					return fmt.Errorf("failed to decode zadig build job, error: %s", err)
				}
				newSpec.TestModuleOptions = newSpec.TestModules
				newSpec.TestModules = make([]*commonmodels.TestModule, 0)
				newSpec.ServiceTestOptions = newSpec.ServiceAndTests
				newSpec.ServiceAndTests = make([]*commonmodels.ServiceAndTest, 0)
				for _, test := range newSpec.TestModuleOptions {
					updateKeyVal(test.KeyVals)
				}

				for _, svc := range newSpec.ServiceTestOptions {
					updateKeyVal(svc.KeyVals)
				}
				job.Spec = newSpec
			default:
			}
		}
	}
	return nil
}

func V340ToV330() error {
	return nil
}

func updateKeyVal(kvs commonmodels.RuntimeKeyValList) {
	for _, kv := range kvs {
		if strings.HasPrefix(kv.Value, "<+fixed>") {
			kv.Source = config.ParamSourceFixed
			kv.Value = strings.TrimPrefix(kv.Value, "<+fixed>")
		} else if strings.HasPrefix(kv.Value, "{{.") {
			kv.Source = config.ParamSourceReference
		} else {
			kv.Source = config.ParamSourceRuntime
		}
	}
}

func converOldFreestyleJobSpec(spec *commonmodels.FreestyleJobSpec) (*commonmodels.FreestyleJobSpec, error) {
	newSpec := &commonmodels.FreestyleJobSpec{
		FreestyleJobType: spec.FreestyleJobType,
		ServiceSource:    spec.ServiceSource,
		JobName:          spec.JobName,
		OriginJobName:    spec.OriginJobName,
		RefRepos:         spec.RefRepos,
	}
	installs := make([]*commonmodels.Item, 0)
	repos := make([]*types.Repository, 0)
	for _, step := range spec.Steps {
		if step.StepType == config.StepTools {
			stepSpec := new(steptype.StepToolInstallSpec)
			if err := commonmodels.IToi(step.Spec, stepSpec); err != nil {
				return nil, fmt.Errorf("failed to decode zadig tool step, error: %s", err)
			}
			for _, install := range stepSpec.Installs {
				installs = append(installs, &commonmodels.Item{
					Name:    install.Name,
					Version: install.Version,
				})
			}
			continue
		}
		if step.StepType == config.StepGit {
			stepSpec := new(steptype.StepGitSpec)
			if err := commonmodels.IToi(step.Spec, stepSpec); err != nil {
				return nil, fmt.Errorf("failed to decode zadig git step, error: %s", err)
			}
			for _, repo := range stepSpec.Repos {
				repos = append(repos, repo)
			}
			continue
		}
		if step.StepType == config.StepPerforce {
			stepSpec := new(steptype.StepP4Spec)
			if err := commonmodels.IToi(step.Spec, stepSpec); err != nil {
				return nil, fmt.Errorf("failed to decode zadig p4 step, error: %s", err)
			}
			for _, repo := range stepSpec.Repos {
				repos = append(repos, repo)
			}
			continue
		}
		if step.StepType == config.StepShell {
			stepSpec := new(steptype.StepShellSpec)
			if err := commonmodels.IToi(step.Spec, stepSpec); err != nil {
				return nil, fmt.Errorf("failed to decode zadig shell step, error: %s", err)
			}
			newSpec.Script = stepSpec.Script
			newSpec.ScriptType = types.ScriptTypeShell
			continue
		}
		if step.StepType == config.StepBatchFile {
			stepSpec := new(steptype.StepBatchFileSpec)
			if err := commonmodels.IToi(step.Spec, stepSpec); err != nil {
				return nil, fmt.Errorf("failed to decode zadig batch file step, error: %s", err)
			}
			newSpec.Script = stepSpec.Script
			newSpec.ScriptType = types.ScriptTypeBatchFile
			continue
		}
		if step.StepType == config.StepPowerShell {
			stepSpec := new(steptype.StepPowerShellSpec)
			if err := commonmodels.IToi(step.Spec, stepSpec); err != nil {
				return nil, fmt.Errorf("failed to decode zadig powershell step, error: %s", err)
			}
			newSpec.Script = stepSpec.Script
			newSpec.ScriptType = types.ScriptTypePowerShell
			continue
		}
		if step.StepType == config.StepArchive {
			stepSpec := new(steptype.StepArchiveSpec)
			if err := commonmodels.IToi(step.Spec, stepSpec); err != nil {
				return nil, fmt.Errorf("failed to decode zadig archive step, error: %s", err)
			}
			uploads := make([]*types.ObjectStoragePathDetail, 0)
			for _, upload := range stepSpec.UploadDetail {
				uploads = append(uploads, &types.ObjectStoragePathDetail{
					FilePath:        upload.FilePath,
					DestinationPath: upload.DestinationPath,
					AbsFilePath:     upload.AbsFilePath,
				})
			}
			newSpec.ObjectStorageUpload = &commonmodels.ObjectStorageUpload{
				Enabled:         true,
				ObjectStorageID: stepSpec.ObjectStorageID,
				UploadDetail:    uploads,
			}
			continue
		}
	}
	defaultServices := make([]*commonmodels.ServiceWithModule, 0)
	for _, svc := range spec.Services {
		defaultServices = append(defaultServices, &commonmodels.ServiceWithModule{
			ServiceName:   svc.ServiceName,
			ServiceModule: svc.ServiceModule,
		})
	}
	newSpec.Repos = repos
	newSpec.Envs = spec.Properties.Envs.ToRuntimeList()
	newSpec.DefaultServices = defaultServices

	runtimeInfo := &commonmodels.RuntimeInfo{
		Infrastructure: spec.Properties.Infrastructure,
		BuildOS:        spec.Properties.BuildOS,
		ImageFrom:      spec.Properties.ImageFrom,
		ImageID:        spec.Properties.ImageID,
		Installs:       installs,
	}
	newSpec.Runtime = runtimeInfo

	commonAdvancedSetting := &commonmodels.JobAdvancedSettings{
		Timeout:             spec.Properties.Timeout,
		ClusterID:           spec.Properties.ClusterID,
		ClusterSource:       spec.Properties.ClusterSource,
		ResourceRequest:     spec.Properties.ResourceRequest,
		ResReqSpec:          spec.Properties.ResReqSpec,
		StrategyID:          spec.Properties.StrategyID,
		UseHostDockerDaemon: spec.Properties.UseHostDockerDaemon,
		CustomAnnotations:   spec.Properties.CustomAnnotations,
		CustomLabels:        spec.Properties.CustomLabels,
		ShareStorageInfo:    spec.Properties.ShareStorageInfo,
	}
	advancedSetting := &commonmodels.FreestyleJobAdvancedSettings{
		JobAdvancedSettings: commonAdvancedSetting,
		Outputs:             spec.Outputs,
	}
	newSpec.AdvancedSetting = advancedSetting
	return newSpec, nil
}
