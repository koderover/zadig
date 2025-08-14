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
	"fmt"
	"strings"

	internalmodels "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/orm"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	collmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	deliveryservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/delivery/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"go.mongodb.org/mongo-driver/bson"
)

func init() {
	upgradepath.RegisterHandler("3.4.1", "4.0.0", V341ToV400)
	upgradepath.RegisterHandler("4.0.0", "3.4.1", V400ToV341)
}

func V341ToV400() error {
	ctx := internalhandler.NewBackgroupContext()

	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}

	defer func() {
		updateMigrationError(migrationInfo.ID, err)
	}()

	err = migrateDeliveryVersionV2(ctx, migrationInfo)
	if err != nil {
		return err
	}

	err = migrateUserGroup(ctx, migrationInfo)
	if err != nil {
		return err
	}

	err = migrateCollaborationInstance(ctx, migrationInfo)
	if err != nil {
		return err
	}

	err = migrateProjectManagement(ctx, migrationInfo)
	if err != nil {
		return err
	}

	return nil
}

func migrateDeliveryVersionV2(ctx *internalhandler.Context, migrationInfo *internalmodels.Migration) error {
	if !migrationInfo.Migration400DeliveryVersionV2 {
		cursor, err := commonrepo.NewDeliveryVersionColl().ListByCursor()
		if err != nil {
			return fmt.Errorf("failed to list delivery versions, err: %s", err)
		}

		for cursor.Next(ctx) {
			var versionV1 models.DeliveryVersion
			err = cursor.Decode(&versionV1)
			if err != nil {
				return fmt.Errorf("failed to decode delivery version, err: %s", err)
			}

			versionV2 := &models.DeliveryVersionV2{
				ProjectName:  versionV1.ProductName,
				Version:      versionV1.Version,
				Type:         setting.DeliveryVersionType(versionV1.Type),
				Source:       setting.DeliveryVersionSourceFromEnv,
				Labels:       versionV1.Labels,
				Desc:         versionV1.Desc,
				WorkflowName: versionV1.WorkflowName,
				Status:       setting.DeliveryVersionStatus(versionV1.Status),
				Error:        versionV1.Error,
				TaskID:       int64(versionV1.TaskID),
				CreatedAt:    versionV1.CreatedAt,
				CreatedBy:    versionV1.CreatedBy,
			}

			if versionV1.ProductEnvInfo != nil {
				versionV2.EnvName = versionV1.ProductEnvInfo.EnvName
				versionV2.Production = versionV1.ProductEnvInfo.Production
			}

			versionV2.Services = make([]*models.DeliveryVersionService, 0)
			if setting.DeliveryVersionType(versionV1.Type) == setting.DeliveryVersionTypeYaml {
				createArgument := &models.DeliveryVersionYamlData{}
				err = models.IToi(versionV1.CreateArgument, createArgument)
				if err != nil {
					return fmt.Errorf("failed to convert create argument, err: %s", err)
				}
				versionV2.ImageRegistryID = createArgument.ImageRegistryID

				for _, yamlData := range createArgument.YamlDatas {
					service := &models.DeliveryVersionService{
						ServiceName: yamlData.ServiceName,
						YamlContent: yamlData.YamlContent,
					}

					for _, imageData := range yamlData.ImageDatas {
						tagArr := strings.Split(imageData.Image, ":")
						if len(tagArr) == 1 {
							return fmt.Errorf("invalid image format: %s", imageData.Image)
						}

						tag := tagArr[len(tagArr)-1]
						service.Images = append(service.Images, &models.DeliveryVersionImage{
							ContainerName:  imageData.ContainerName,
							ImageName:      imageData.ImageName,
							SourceImage:    imageData.Image,
							SourceImageTag: tag,
							TargetImage:    imageData.Image,
							TargetImageTag: tag,
							PushImage:      imageData.Selected,
						})
					}

					versionV2.Services = append(versionV2.Services, service)
				}
			} else if setting.DeliveryVersionType(versionV1.Type) == setting.DeliveryVersionTypeChart {
				createArgument := &models.DeliveryVersionChartData{}
				err = models.IToi(versionV1.CreateArgument, createArgument)
				if err != nil {
					return fmt.Errorf("failed to convert create argument, err: %s", err)
				}
				versionV2.ImageRegistryID = createArgument.ImageRegistryID
				versionV2.ChartRepoName = createArgument.ChartRepoName

				distributes, err := commonrepo.NewDeliveryDistributeColl().Find(&commonrepo.DeliveryDistributeArgs{
					ReleaseID:      versionV1.ID.Hex(),
					DistributeType: config.Chart,
				})
				if err != nil {
					return fmt.Errorf("failed to find delivery distribute, err: %s", err)
				}

				successChartMap := make(map[string]bool)
				for _, distribute := range distributes {
					successChartMap[distribute.ChartName] = true
				}

				for _, chartData := range createArgument.ChartDatas {
					service := &models.DeliveryVersionService{
						ServiceName:          chartData.ServiceName,
						ChartName:            chartData.ServiceName,
						OriginalChartVersion: chartData.Version,
						ChartVersion:         chartData.Version,
						YamlContent:          chartData.ValuesYamlContent,
					}

					if successChartMap[chartData.ServiceName] {
						service.ChartStatus = config.StatusPassed
					} else {
						service.ChartStatus = config.StatusFailed
					}

					for _, imageData := range chartData.ImageData {
						tagArr := strings.Split(imageData.Image, ":")
						if len(tagArr) == 1 {
							return fmt.Errorf("invalid image format: %s", imageData.Image)
						}

						tag := tagArr[len(tagArr)-1]
						image := &models.DeliveryVersionImage{
							ContainerName:  imageData.ImageName,
							ImageName:      imageData.ImageName,
							SourceImage:    imageData.Image,
							SourceImageTag: tag,
							TargetImage:    imageData.Image,
							TargetImageTag: tag,
							PushImage:      imageData.Selected,
						}

						imagePath := &models.ImagePathSpec{}
						if versionV1.ProductEnvInfo != nil {
							for _, serviceGroup := range versionV1.ProductEnvInfo.Services {
								for _, service := range serviceGroup {
									if service.ServiceName == chartData.ServiceName {
										for _, container := range service.Containers {
											if container.Name == imageData.ImageName {
												imagePath = container.ImagePath
												goto EndLoop
											}
										}
									}
								}
							}
						}

					EndLoop:
						image.ImagePath = imagePath

						service.Images = append(service.Images, image)
					}

					versionV2.Services = append(versionV2.Services, service)
				}
			} else {
				return fmt.Errorf("unsupported delivery version type: %s", versionV1.Type)
			}

			if versionV1.WorkflowName != "" && versionV1.TaskID != 0 {
				task, err := commonrepo.NewworkflowTaskv4Coll().Find(versionV1.WorkflowName, int64(versionV1.TaskID))
				if err != nil {
					return fmt.Errorf("failed to find workflow task %s/%d err: %s", versionV1.WorkflowName, task.TaskID, err)
				}

				_, err = deliveryservice.CheckDeliveryImageStatus(versionV2, task, log.SugaredLogger())
				if err != nil {
					return fmt.Errorf("failed to check delivery image status, err: %v", err)
				}
			}

			err = commonrepo.NewDeliveryVersionV2Coll().Create(versionV2)
			if err != nil {
				return fmt.Errorf("failed to create delivery version v2, projectName: %s, version: %s, err: %s", versionV2.ProjectName, versionV2.Version, err)
			}
		}
	}

	_ = internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration400DeliveryVersionV2): true,
	})

	return nil
}

func migrateUserGroup(ctx *internalhandler.Context, migrationInfo *internalmodels.Migration) error {
	if !migrationInfo.Migration400AllUserGroup {
		err := orm.UpdateAllUserGroup()
		if err != nil {
			return fmt.Errorf("failed to update all user group, err: %s", err)
		}
	}

	_ = internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration400AllUserGroup): true,
	})

	return nil
}

func V400ToV341() error {
	return nil
}

func migrateCollaborationInstance(ctx *internalhandler.Context, migrationInfo *internalmodels.Migration) error {
	if !migrationInfo.Migration400CollaborationInstance {
		cursor, err := mongodb.NewCollaborationInstanceColl().ListByCursor()
		if err != nil {
			return fmt.Errorf("failed to list delivery versions, err: %s", err)
		}

		for cursor.Next(ctx) {
			var instance collmodels.CollaborationInstance
			err = cursor.Decode(&instance)
			if err != nil {
				return fmt.Errorf("failed to decode collaboration instance, err: %s", err)
			}

			if instance.IsDeleted {
				continue
			}

			_, err = mongodb.NewCollaborationInstanceColl().UpdateOne(ctx, bson.M{"_id": instance.ID}, bson.M{"$set": bson.M{"is_deleted": false}})
			if err != nil {
				return fmt.Errorf("failed to update collaboration instance, err: %s", err)
			}
		}
	}

	_ = internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration400CollaborationInstance): true,
	})

	return nil
}

func migrateProjectManagement(ctx *internalhandler.Context, migrationInfo *internalmodels.Migration) error {
	if !migrationInfo.Migration400ProjectManagement {
		pms, err := commonrepo.NewProjectManagementColl().List()
		if err != nil {
			return fmt.Errorf("failed to list project management, err: %s", err)
		}

		for _, pm := range pms {
			if pm.Type == setting.ProjectManagementTypeJira {
				pm.Spec = &models.ProjectManagementJiraSpec{
					JiraAuthType:            pm.JiraAuthType,
					JiraHost:                pm.JiraHost,
					JiraUser:                pm.JiraUser,
					JiraToken:               pm.JiraToken,
					JiraPersonalAccessToken: pm.JiraPersonalAccessToken,
				}

				pm.JiraAuthType = ""
				pm.JiraHost = ""
				pm.JiraUser = ""
				pm.JiraToken = ""
				pm.JiraPersonalAccessToken = ""
			} else if pm.Type == setting.ProjectManagementTypeMeego {
				pm.Spec = &models.ProjectManagementMeegoSpec{
					MeegoHost:         pm.MeegoHost,
					MeegoPluginID:     pm.MeegoPluginID,
					MeegoPluginSecret: pm.MeegoPluginSecret,
					MeegoUserKey:      pm.MeegoUserKey,
				}

				pm.MeegoHost = ""
				pm.MeegoPluginID = ""
				pm.MeegoPluginSecret = ""
				pm.MeegoUserKey = ""
			}

			err = commonrepo.NewProjectManagementColl().UpdateByID(pm.ID.Hex(), pm)
			if err != nil {
				return fmt.Errorf("failed to update project management, err: %s", err)
			}
		}
	}

	_ = internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration400ProjectManagement): true,
	})

	return nil
}
