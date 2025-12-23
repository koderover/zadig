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

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	internalmodels "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	upgradepath.RegisterHandler("4.1.0", "4.2.0", V410ToV420)
	upgradepath.RegisterHandler("4.2.0", "4.1.0", V420ToV410)
}

func V410ToV420() error {
	ctx := internalhandler.NewBackgroupContext()

	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}

	defer func() {
		updateMigrationError(migrationInfo.ID, err)
	}()

	err = migrateVMDeployJob(ctx, migrationInfo)
	if err != nil {
		return err
	}

	err = migrateVMDeploy(ctx, migrationInfo)
	if err != nil {
		return err
	}

	return nil
}

func migrateVMDeploy(ctx *internalhandler.Context, migrationInfo *internalmodels.Migration) error {
	if !migrationInfo.Migration420VMDeploy {
		vmProjects, err := templaterepo.NewProductColl().ListWithOption(&templaterepo.ProductListOpt{
			BasicFacility: setting.BasicFacilityCVM,
		})
		if err != nil {
			return fmt.Errorf("failed to list projects, err: %s", err)
		}
		for _, project := range vmProjects {
			builds, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{
				ProductName: project.ProductName,
			})
			if err != nil {
				return fmt.Errorf("failed to list builds, err: %s", err)
			}

			for _, build := range builds {
				if len(build.Targets) == 0 || build.Targets[0].ServiceName == "" {
					continue
				}

				deploy := &commonmodels.Deploy{
					Name:                     fmt.Sprintf("%s-deploy", build.Targets[0].ServiceName),
					ProjectName:              project.ProductName,
					ServiceName:              build.Targets[0].ServiceName,
					Source:                   build.Source,
					Timeout:                  build.Timeout,
					Description:              build.Description,
					UpdateTime:               build.UpdateTime,
					UpdateBy:                 build.UpdateBy,
					Repos:                    build.DeployRepos,
					Infrastructure:           build.DeployInfrastructure,
					VMLabels:                 build.DeployVMLabels,
					ScriptType:               build.PMDeployScriptType,
					Scripts:                  build.PMDeployScripts,
					SSHs:                     build.SSHs,
					PreDeploy:                build.PreDeploy,
					Type:                     build.DeployType,
					ArtifactType:             build.DeployArtifactType,
					AdvancedSettingsModified: build.AdvancedSettingsModified,
					Outputs:                  build.Outputs,
				}

				deploy.PreDeploy.ResReq = build.PreBuild.ResReq
				deploy.PreDeploy.ResReqSpec = build.PreBuild.ResReqSpec
				deploy.PreDeploy.ClusterID = build.PreBuild.ClusterID
				deploy.PreDeploy.StrategyID = build.PreBuild.StrategyID

				exist, err := commonrepo.NewDeployColl().Find(&commonrepo.DeployFindOption{
					ProjectName: deploy.ProjectName,
					ServiceName: deploy.ServiceName,
				})
				if err != nil {
					if err != mongo.ErrNoDocuments {
						log.Errorf("check deploy exist %s/%s failed, err: %s", deploy.ProjectName, deploy.ServiceName, err)
						continue
					}
				}
				if exist != nil {
					log.Warnf("deploy %s/%s already existed", deploy.ProjectName, deploy.ServiceName)
					continue
				}

				err = commonrepo.NewDeployColl().Create(deploy)
				if err != nil {
					log.Errorf("failed to create deploy, project %s, service %s, err: %s", project.ProductName, build.Targets[0].ServiceName, err)
					continue
				}
			}
		}
	}

	_ = internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration420VMDeploy): true,
	})

	return nil
}

func migrateVMDeployJob(ctx *internalhandler.Context, migrationInfo *models.Migration) error {
	if !migrationInfo.Migration420VMDeployEnvSource {
		count := 0
		vmProjects, err := templaterepo.NewProductColl().ListWithOption(&templaterepo.ProductListOpt{
			BasicFacility: setting.BasicFacilityCVM,
		})
		if err != nil {
			return fmt.Errorf("failed to list all vm projects to migrate, error: %s", err)
		}

		for _, vmProject := range vmProjects {
			workflowCursor, err := commonrepo.NewWorkflowV4Coll().ListByCursor(&commonrepo.ListWorkflowV4Option{ProjectName: vmProject.ProductName})
			if err != nil {
				return fmt.Errorf("failed to list all custom workflow to update, error: %s", err)
			}

			for workflowCursor.Next(context.Background()) {
				workflow := new(commonmodels.WorkflowV4)
				if err := workflowCursor.Decode(workflow); err != nil {
					// continue converting to have maximum converage
					log.Errorf("failed to decode workflow: %s in project %s, error: %s", workflow.Name, workflow.Project, err)
					continue
				}

				changed := false
				for _, stage := range workflow.Stages {
					for _, job := range stage.Jobs {
						if job.JobType == config.JobZadigVMDeploy {
							newSpec := new(commonmodels.ZadigVMDeployJobSpec)
							if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
								log.Errorf("failed to decode zadig vm deploy job: %s in workflow: %s in project %s, error: %s", job.Name, workflow.Name, workflow.Project, err)
								continue
							}

							newSpec.EnvSource = config.ParamSourceRuntime
							job.Spec = newSpec
							changed = true
							count++
						}
					}
				}

				if changed {
					err = commonrepo.NewWorkflowV4Coll().Update(
						workflow.ID.Hex(),
						workflow,
					)

					if err != nil {
						log.Warnf("failed to update workflow: %s in project %s, error: %s", workflow.Name, workflow.Project, err)
					}
				}

			}
		}

		log.Infof("migrated %d vm deploy workflows", count)

		_ = internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
			getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration420VMDeployEnvSource): true,
		})
	}

	return nil
}

func V420ToV410() error {
	return nil
}
