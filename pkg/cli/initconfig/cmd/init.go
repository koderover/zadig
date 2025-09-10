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

package cmd

import (
	"context"
	_ "embed"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/koderover/zadig/v2/pkg/config"
	modeMongodb "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/ai"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	vmcommonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/vm"
	statrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
	systemrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/repository/mongodb"
	systemservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	templateservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/templatestore/service"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	configmongodb "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/email/repository/mongodb"
	userdb "github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/client/aslan"
	gormtool "github.com/koderover/zadig/v2/pkg/tool/gorm"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"

	aslanconfig "github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"

	"errors"

	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	rootCmd.AddCommand(initCmd)
	log.Init(&log.Config{
		Level: config.LogLevel(),
	})
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "init system config",
	Long:  `init system config.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(); err != nil {
			log.Fatal(err)
		}
	},
}

type indexer interface {
	EnsureIndex(ctx context.Context) error
	GetCollectionName() string
}

func run() error {
	// initialize connection to both databases
	err := gormtool.Open(config.MysqlUser(),
		config.MysqlPassword(),
		config.MysqlHost(),
		config.MysqlDexDB(),
	)
	if err != nil {
		log.Panicf("Failed to open database %s", config.MysqlDexDB())
	}

	err = gormtool.Open(config.MysqlUser(),
		config.MysqlPassword(),
		config.MysqlHost(),
		config.MysqlUserDB(),
	)
	if err != nil {
		log.Panicf("Failed to open database %s", config.MysqlUserDB())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// mongodb initialization
	mongotool.Init(ctx, config.MongoURI())
	if err := mongotool.Ping(ctx); err != nil {
		panic(fmt.Errorf("failed to connect to mongo, error: %s", err))
	}

	createOrUpdateMongodbIndex(ctx)

	err = initSystemData()
	if err == nil {
		log.Info("zadig init success")
	}

	return err
}

func createOrUpdateMongodbIndex(ctx context.Context) {
	var wg sync.WaitGroup
	for _, r := range []indexer{
		// aslan related db index
		template.NewProductColl(),
		commonrepo.NewApplicationColl(),
		commonrepo.NewApplicationFieldDefinitionColl(),
		commonrepo.NewBasicImageColl(),
		commonrepo.NewBuildColl(),
		commonrepo.NewCallbackRequestColl(),
		commonrepo.NewCICDToolColl(),
		commonrepo.NewConfigurationManagementColl(),
		commonrepo.NewCounterColl(),
		commonrepo.NewCronjobColl(),
		commonrepo.NewCustomWorkflowTestReportColl(),
		commonrepo.NewDeliveryActivityColl(),
		commonrepo.NewDeliveryArtifactColl(),
		commonrepo.NewDeliveryDeployColl(),
		commonrepo.NewDeliveryDistributeColl(),
		commonrepo.NewDeliveryVersionV2Coll(),
		commonrepo.NewDiffNoteColl(),
		commonrepo.NewDindCleanColl(),
		commonrepo.NewIMAppColl(),
		commonrepo.NewObservabilityColl(),
		commonrepo.NewFavoriteColl(),
		commonrepo.NewGithubAppColl(),
		commonrepo.NewHelmRepoColl(),
		commonrepo.NewInstallColl(),
		commonrepo.NewItReportColl(),
		commonrepo.NewK8SClusterColl(),
		commonrepo.NewNotificationColl(),
		commonrepo.NewNotifyColl(),
		commonrepo.NewPipelineColl(),
		commonrepo.NewPrivateKeyColl(),
		commonrepo.NewProductColl(),
		commonrepo.NewProxyColl(),
		commonrepo.NewQueueColl(),
		commonrepo.NewRegistryNamespaceColl(),
		commonrepo.NewS3StorageColl(),
		commonrepo.NewServiceColl(),
		commonrepo.NewProductionServiceColl(),
		commonrepo.NewStrategyColl(),
		commonrepo.NewStatsColl(),
		commonrepo.NewSubscriptionColl(),
		commonrepo.NewSystemSettingColl(),
		commonrepo.NewTaskColl(),
		commonrepo.NewTestTaskStatColl(),
		commonrepo.NewTestingColl(),
		commonrepo.NewWebHookColl(),
		commonrepo.NewWebHookUserColl(),
		commonrepo.NewWorkflowColl(),
		commonrepo.NewWorkflowStatColl(),
		commonrepo.NewExternalLinkColl(),
		commonrepo.NewChartColl(),
		commonrepo.NewDockerfileTemplateColl(),
		commonrepo.NewProjectClusterRelationColl(),
		commonrepo.NewEnvResourceColl(),
		commonrepo.NewEnvSvcDependColl(),
		commonrepo.NewBuildTemplateColl(),
		commonrepo.NewScanningColl(),
		commonrepo.NewWorkflowV4Coll(),
		commonrepo.NewworkflowTaskv4Coll(),
		commonrepo.NewWorkflowQueueColl(),
		commonrepo.NewPluginRepoColl(),
		commonrepo.NewWorkflowViewColl(),
		commonrepo.NewWorkflowV4TemplateColl(),
		commonrepo.NewVariableSetColl(),
		commonrepo.NewJobInfoColl(),
		commonrepo.NewStatDashboardConfigColl(),
		commonrepo.NewProjectManagementColl(),
		commonrepo.NewImageTagsCollColl(),
		commonrepo.NewLLMIntegrationColl(),
		commonrepo.NewReleasePlanColl(),
		commonrepo.NewReleasePlanLogColl(),
		commonrepo.NewEnvServiceVersionColl(),
		commonrepo.NewLabelColl(),
		commonrepo.NewSprintTemplateColl(),
		commonrepo.NewSprintColl(),
		commonrepo.NewSprintWorkItemColl(),
		commonrepo.NewSprintWorkItemTaskColl(),
		commonrepo.NewSprintWorkItemActivityColl(),
		commonrepo.NewLabelBindingColl(),
		commonrepo.NewSAEColl(),
		commonrepo.NewSAEEnvColl(),
		commonrepo.NewEnvInfoColl(),
		commonrepo.NewApprovalTicketColl(),
		commonrepo.NewWorkflowTaskRevertColl(),

		// msg queue
		commonrepo.NewMsgQueueCommonColl(),
		commonrepo.NewMsgQueuePipelineTaskColl(),

		systemrepo.NewAnnouncementColl(),
		systemrepo.NewOperationLogColl(),
		modeMongodb.NewCollaborationModeColl(),
		modeMongodb.NewCollaborationInstanceColl(),

		// config related db index
		configmongodb.NewEmailHostColl(),

		// user related db index
		userdb.NewUserSettingColl(),

		// env AI analysis related db index
		ai.NewEnvAIAnalysisColl(),

		// project group related db index
		commonrepo.NewProjectGroupColl(),

		// db instances
		commonrepo.NewDBInstanceColl(),

		// vm job related db index
		vmcommonrepo.NewVMJobColl(),

		statrepo.NewWeeklyDeployStatColl(),
		statrepo.NewMonthlyDeployStatColl(),
		statrepo.NewMonthlyReleaseStatColl(),
	} {
		wg.Add(1)
		go func(r indexer) {
			defer wg.Done()
			if err := r.EnsureIndex(ctx); err != nil {
				panic(fmt.Errorf("failed to create index for %s, error: %s", r.GetCollectionName(), err))
			}
		}(r)
	}

	wg.Wait()
}

func initSystemData() error {
	if err := commonrepo.NewSystemSettingColl().InitSystemSettings(); err != nil {
		log.Errorf("initialize system settings err:%s", err)
		return err
	}

	if err := commonrepo.NewS3StorageColl().InitData(); err != nil {
		log.Warnf("Failed to init S3 data: %s", err)
	}

	commonrepo.NewBasicImageColl().InitBasicImageData(systemservice.InitbasicImageInfos())

	if err := commonrepo.NewInstallColl().InitInstallData(systemservice.InitInstallMap()); err != nil {
		log.Errorf("initialize Install Data err:%s", err)
		return err
	}

	// Seed built-in application field definitions (idempotent)
	if err := createBuiltinApplicationFieldDefinitions(); err != nil {
		log.Errorf("failed to create builtin application field definitions: %s", err)
		return err
	}

	if err := createLocalCluster(); err != nil {
		log.Errorf("createLocalCluster err:%s", err)
		return err
	}

	templateservice.InitWorkflowTemplate()

	// update offical plugins
	workflowservice.UpdateOfficalPluginRepository(log.SugaredLogger())

	if err := clearSharedStorage(); err != nil {
		log.Errorf("failed to clear aslan shared storage, error: %s", err)
	}
	return nil
}

// createBuiltinApplicationFieldDefinitions creates built-in Application fields into the
// application_field_definition collection.
func createBuiltinApplicationFieldDefinitions() error {
	coll := commonrepo.NewApplicationFieldDefinitionColl()
	ctx := context.Background()

	// Define built-in fields. Ignore _id and plugins; treat repository as a special type without inner-field visibility.
	builtin := []commonmodels.ApplicationFieldDefinition{
		{Key: "name", Name: "名称", Type: aslanconfig.ApplicationCustomFieldTypeText, Required: true, ShowInList: true, Source: aslanconfig.ApplicationFieldSourceBuiltin, Description: "业务服务的名称"},
		{Key: "key", Name: "标识", Type: aslanconfig.ApplicationCustomFieldTypeText, Required: true, ShowInList: true, Source: aslanconfig.ApplicationFieldSourceBuiltin, Description: "业务服务的唯一标识符"},
		{Key: "project", Name: "项目归属", Type: aslanconfig.ApplicationCustomFieldTypeProject, Required: true, ShowInList: true, Source: aslanconfig.ApplicationFieldSourceBuiltin, Description: "业务服务所属的项目"},
		{Key: "type", Name: "类型", Type: aslanconfig.ApplicationCustomFieldTypeSingleSelect, ShowInList: true, Source: aslanconfig.ApplicationFieldSourceBuiltin, Description: "业务服务的类型", Options: []string{"服务", "组件", "应用"}},
		{Key: "owner", Name: "负责人", Type: aslanconfig.ApplicationCustomFieldTypeUser, ShowInList: true, Source: aslanconfig.ApplicationFieldSourceBuiltin, Description: "业务服务的负责人"},
		{Key: "repository", Name: "代码库", Type: aslanconfig.ApplicationCustomFieldTypeRepository, ShowInList: true, Source: aslanconfig.ApplicationFieldSourceBuiltin, Description: "代码仓库"},
		{Key: "description", Name: "描述", Type: aslanconfig.ApplicationCustomFieldTypeText, ShowInList: true, Source: aslanconfig.ApplicationFieldSourceBuiltin, Description: "业务服务的详细描述"},
		{Key: "testing_service_name", Name: "测试配置", Type: aslanconfig.ApplicationCustomFieldTypeServiceConfig, ShowInList: true, Source: aslanconfig.ApplicationFieldSourceBuiltin, Description: "测试环境的配置"},
		{Key: "production_service_name", Name: "生产配置", Type: aslanconfig.ApplicationCustomFieldTypeServiceConfig, ShowInList: true, Source: aslanconfig.ApplicationFieldSourceBuiltin, Description: "生产环境的配置"},
		{Key: "create_time", Name: "创建时间", Type: aslanconfig.ApplicationCustomFieldTypeDatetime, ShowInList: true, Source: aslanconfig.ApplicationFieldSourceBuiltin, Description: "业务服务的创建时间"},
		{Key: "update_time", Name: "更新时间", Type: aslanconfig.ApplicationCustomFieldTypeDatetime, ShowInList: true, Source: aslanconfig.ApplicationFieldSourceBuiltin, Description: "业务服务的更新时间"},
	}

	// Upsert per key to be idempotent. Keep user-changed attributes for custom fields; for built-ins we only enforce Source="builtin" and Type.
	for i := range builtin {
		b := builtin[i]
		existing, err := coll.GetByKey(ctx, b.Key)
		if err != nil {
			// if not found, create
			if errors.Is(err, mongo.ErrNoDocuments) {
				// set timestamps via Create
				if _, cerr := coll.Create(ctx, &b); cerr != nil {
					return cerr
				}
				continue
			}
			return err
		}
		// Already exists: only update Source, Type, Name, Description, Required if they differ; preserve other settings (e.g., ShowInList) to avoid surprising overrides.
		needUpdate := false
		if existing.Source != aslanconfig.ApplicationFieldSourceBuiltin {
			existing.Source = aslanconfig.ApplicationFieldSourceBuiltin
			needUpdate = true
		}
		if existing.Type != b.Type {
			existing.Type = b.Type
			needUpdate = true
		}
		if existing.Name != b.Name {
			existing.Name = b.Name
			needUpdate = true
		}
		if existing.Description != b.Description {
			existing.Description = b.Description
			needUpdate = true
		}
		if existing.Required != b.Required {
			existing.Required = b.Required
			needUpdate = true
		}
		if needUpdate {
			if err := coll.UpdateByID(ctx, existing.ID.Hex(), existing); err != nil {
				return err
			}
		}
	}
	return nil
}

func createLocalCluster() error {
	cluster, err := aslan.New(config.AslanServiceAddress()).GetLocalCluster()
	if err != nil {
		return err
	}
	if cluster != nil {
		return nil
	}
	return aslan.New(config.AslanServiceAddress()).AddLocalCluster()
}

func clearSharedStorage() error {
	return aslan.New(config.AslanServiceAddress()).ClearSharedStorage()
}
