/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package migrate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	aslanConfig "github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	systemservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
)

func init() {
	upgradepath.RegisterHandler("1.17.0", "1.18.0", V1170ToV1180)
	upgradepath.RegisterHandler("1.18.0", "1.17.0", V1180ToV1170)
}

func V1170ToV1180() error {
	if err := migrateJiraAuthType(); err != nil {
		log.Errorf("migrateJiraAuthType err: %v", err)
		return errors.Wrapf(err, "failed to execute migrateJiraAuthType")
	}
	if err := migrateSystemTheme(); err != nil {
		log.Errorf("migrateSystemTheme err: %v", err)
		return errors.Wrapf(err, "failed to execute migrateSystemTheme")
	}

	log.Info("-------- start migrate variables --------")
	if err := migrateVariables(); err != nil {
		log.Errorf("migrateVariables err: %v", err)
		return errors.Wrapf(err, "failed to execute migrateVariables")
	}

	log.Info("-------- start migrate workflow v4 data --------")
	if err := migrateWorkflowV4Data(); err != nil {
		log.Errorf("migrateWorkflowV4Data err: %v", err)
		return err
	}

	log.Info("-------- start migrate external product --------")
	if err := migrateExternalProductIsExisted(); err != nil {
		log.Errorf("migrateExternalProductIsExisted err: %v", err)
		return err
	}

	log.Info("-------- start migrate workflow concurrency limit --------")
	if err := migrateWorkflowV4ConcurrencyLimit(); err != nil {
		log.Errorf("migrateWorkflowV4ConcurrencyLimit err: %v", err)
		return err
	}

	log.Info("-------- start migrate service module fields --------")
	if err := migrateServiceModulesFieldForWorkflowV4Task(); err != nil {
		log.Errorf("migrateServiceModulesFieldForWorkflowV4Task err: %v", err)
		return errors.Wrapf(err, "failed to execute migrateServiceModulesFieldForWorkflowV4Task")
	}

	log.Infof("-------- start migrate project name pinyin --------")
	if err := migrateProjectNamePinyin(); err != nil {
		log.Errorf("migrateProjectNamePinyin err: %v", err)
		return err
	}

	log.Infof("-------- start migrate package dependencies --------")
	if err := updatePackageDependencies(); err != nil {
		log.Errorf("updatePackageDependencies err: %v", err)
		return err
	}

	return nil
}

func V1180ToV1170() error {
	return nil
}

func migrateJiraAuthType() error {
	if jira, err := mongodb.NewProjectManagementColl().GetJira(); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil
		}
		return errors.Wrap(err, "get jira")
	} else {
		if jira.JiraAuthType != "" {
			log.Warnf("migrateJiraAuthType: find jira auth type %s, skip", jira.JiraAuthType)
			return nil
		}
		jira.JiraAuthType = config.JiraBasicAuth
		if err := mongodb.NewProjectManagementColl().UpdateByID(jira.ID.Hex(), jira); err != nil {
			return errors.Wrap(err, "update")
		}
	}
	return nil
}

func migrateSystemTheme() error {
	mdb := mongodb.NewSystemSettingColl()
	if systemSetting, err := mdb.Get(); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil
		}
		return errors.Wrap(err, "get system setting")
	} else {
		if systemSetting.Theme != nil {
			return nil
		}
		theme := &models.Theme{
			ThemeType: aslanConfig.CUSTOME_THEME,
			CustomTheme: &models.CustomTheme{
				BorderGray:               "#d2d7dc",
				FontGray:                 "#888888",
				FontLightGray:            "#a0a0a0",
				ThemeColor:               "#0066ff",
				ThemeBorderColor:         "#66bbff",
				ThemeBackgroundColor:     "#eeeeff",
				ThemeLightColor:          "#66bbff",
				BackgroundColor:          "#e5e5e5",
				GlobalBackgroundColor:    "#f6f6f6",
				Success:                  "#67c23a",
				Danger:                   "#f56c6c",
				Warning:                  "#e6a23c",
				Info:                     "#909399",
				Primary:                  "#0066ff",
				WarningLight:             "#cdb62c",
				NotRunning:               "#303133",
				PrimaryColor:             "#000",
				SecondaryColor:           "#888888",
				SidebarBg:                "#f5f7fa",
				SidebarActiveColor:       "#0066ff12",
				ProjectItemIconColor:     "#0066ff",
				ProjectNameColor:         "#121212",
				TableCellBackgroundColor: "#eaeaea",
				LinkColor:                "#0066ff",
			},
		}
		err := mdb.UpdateTheme(theme)
		if err != nil {
			return err
		}
	}
	return nil
}

func createDefaultLarkApprovalDefinition(larks []*models.IMApp) error {
	for _, info := range larks {
		if err := lark.Validate(info.AppID, info.AppSecret); err != nil {
			log.Warnf("lark app %s validate err: %v", info.AppID, err)
			continue
		}
		cli := lark.NewClient(info.AppID, info.AppSecret)
		approvalCode, err := cli.CreateApprovalDefinition(&lark.CreateApprovalDefinitionArgs{
			Name:        "Zadig 工作流",
			Description: "Zadig 工作流-OR",
			Nodes: []*lark.ApprovalNode{
				{
					Type: lark.ApproveTypeOr,
				},
			},
		})
		if err != nil {
			return errors.Wrapf(err, "create lark approval definition %s", info.AppID)
		}
		err = cli.SubscribeApprovalDefinition(&lark.SubscribeApprovalDefinitionArgs{ApprovalID: approvalCode})
		if err != nil {
			return errors.Wrapf(err, "subscribe lark approval definition %s", info.AppID)
		}
		if info.LarkApprovalCodeList == nil {
			info.LarkApprovalCodeList = make(map[string]string)
		}
		info.LarkApprovalCodeList[string(lark.ApproveTypeOr)] = approvalCode
		if err = mongodb.NewIMAppColl().Update(context.Background(), info.ID.Hex(), info); err != nil {
			return errors.Wrapf(err, "update lark app %s", info.AppID)
		}
		log.Infof("create lark approval definition %s success, code %s", info.AppID, approvalCode)
	}
	return nil
}

// migrateWorkflowV4LarkApproval and migrateWorkflowV4DeployTarget
func migrateWorkflowV4Data() error {
	log := log.SugaredLogger().With("func", "migrateWorkflowV4Data")
	larkApps, err := mongodb.NewIMAppColl().List(context.Background(), setting.IMLark)
	switch err {
	case mongo.ErrNoDocuments:
	case nil:
		if err = createDefaultLarkApprovalDefinition(larkApps); err != nil {
			return errors.Wrap(err, "create default lark approval definition")
		}
	default:
		return errors.Wrap(err, "list lark apps")
	}

	cursor, err := mongodb.NewWorkflowV4Coll().ListByCursor(&mongodb.ListWorkflowV4Option{})
	if err != nil {
		return err
	}
	var ms []mongo.WriteModel
	for cursor.Next(context.Background()) {
		var workflow models.WorkflowV4
		if err := cursor.Decode(&workflow); err != nil {
			return err
		}
		if setLarkApprovalNodeForWorkflowV4(&workflow) || setCustomDeployTargetForWorkflowV4(&workflow) {
			ms = append(ms,
				mongo.NewUpdateOneModel().
					SetFilter(bson.D{{"_id", workflow.ID}}).
					SetUpdate(bson.D{{"$set",
						bson.D{
							{"stages", workflow.Stages},
						}},
					}),
			)
			log.Infof("add workflowV4 %s", workflow.Name)
		}
		if len(ms) >= 50 {
			log.Infof("update %d workflowV4", len(ms))
			if _, err := mongodb.NewWorkflowV4Coll().BulkWrite(context.TODO(), ms); err != nil {
				return fmt.Errorf("update workflowV4s error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		log.Infof("update %d workflowV4s", len(ms))
		if _, err := mongodb.NewWorkflowV4Coll().BulkWrite(context.TODO(), ms); err != nil {
			return fmt.Errorf("udpate workflowV4s error: %s", err)
		}
	}

	taskCursor, err := mongodb.NewworkflowTaskv4Coll().ListByCursor(&mongodb.ListWorkflowTaskV4Option{})
	if err != nil {
		return err
	}
	var mTasks []mongo.WriteModel
	for taskCursor.Next(context.Background()) {
		var workflowTask models.WorkflowTask
		if err := taskCursor.Decode(&workflowTask); err != nil {
			return err
		}
		if setLarkApprovalNodeForWorkflowV4Task(&workflowTask) || setLarkApprovalNodeForWorkflowV4(workflowTask.OriginWorkflowArgs) {
			mTasks = append(mTasks,
				mongo.NewUpdateOneModel().
					SetFilter(bson.D{{"_id", workflowTask.ID}}).
					SetUpdate(bson.D{{"$set",
						bson.D{
							{"stages", workflowTask.Stages},
							{"origin_workflow_args", workflowTask.OriginWorkflowArgs},
						}},
					}),
			)
			log.Infof("add workflowV4 task %s", workflowTask.WorkflowName)
		}

		if len(mTasks) >= 50 {
			log.Infof("update %d workflowv4 tasks", len(mTasks))
			if _, err := mongodb.NewworkflowTaskv4Coll().BulkWrite(context.TODO(), mTasks); err != nil {
				return fmt.Errorf("udpate workflowV4 tasks error: %s", err)
			}
			mTasks = []mongo.WriteModel{}
		}
	}
	if len(mTasks) > 0 {
		log.Infof("update %d workflowv4 tasks", len(mTasks))
		if _, err := mongodb.NewworkflowTaskv4Coll().BulkWrite(context.TODO(), mTasks); err != nil {
			return fmt.Errorf("udpate workflowV4 tasks error: %s", err)
		}
	}

	return nil
}

func setLarkApprovalNodeForWorkflowV4(workflow *models.WorkflowV4) (updated bool) {
	if workflow == nil {
		return false
	}
	for _, stage := range workflow.Stages {
		if stage.Approval != nil && stage.Approval.LarkApproval != nil {
			if len(stage.Approval.LarkApproval.ApprovalNodes) > 0 {
				continue
			}
			if len(stage.Approval.LarkApproval.ApproveUsers) == 0 {
				continue
			}
			stage.Approval.LarkApproval.ApprovalNodes = []*models.LarkApprovalNode{
				{
					ApproveUsers: stage.Approval.LarkApproval.ApproveUsers,
					Type:         lark.ApproveTypeOr,
				},
			}
			updated = true
		}
	}
	return updated
}

func setCustomDeployTargetForWorkflowV4(workflow *models.WorkflowV4) (updated bool) {
	if workflow == nil {
		return false
	}
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType != aslanConfig.JobCustomDeploy {
				continue
			}
			jobSpec := &models.CustomDeployJobSpec{}
			if err := models.IToi(job.Spec, jobSpec); err != nil {
				log.Errorf("failed to convert job spec to custom deploy job spec, err: %v", err)
				continue
			}
			for _, target := range jobSpec.Targets {
				if len(target.ImageName) > 0 {
					continue
				}
				curTargetStr := strings.Split(target.Target, "/")
				if len(curTargetStr) == 3 {
					target.ImageName = curTargetStr[2]
				}
				updated = true
			}
			job.Spec = jobSpec
		}
	}
	return updated
}

func setLarkApprovalNodeForWorkflowV4Task(workflow *models.WorkflowTask) (updated bool) {
	if workflow == nil {
		return false
	}
	for _, stage := range workflow.Stages {
		if stage.Approval != nil && stage.Approval.LarkApproval != nil && len(stage.Approval.LarkApproval.ApproveUsers) > 0 {
			if len(stage.Approval.LarkApproval.ApprovalNodes) > 0 {
				continue
			}
			if len(stage.Approval.LarkApproval.ApproveUsers) == 0 {
				continue
			}
			stage.Approval.LarkApproval.ApprovalNodes = []*models.LarkApprovalNode{
				{
					ApproveUsers: stage.Approval.LarkApproval.ApproveUsers,
					Type:         lark.ApproveTypeOr,
				},
			}
			updated = true
		}
	}
	return updated
}

func migrateExternalProductIsExisted() error {
	_, err := mongodb.NewProductColl().UpdateMany(context.Background(),
		bson.M{"source": "external"}, bson.M{"$set": bson.M{"is_existed": true}})
	return err
}

func migrateProductEnvAnalysisConfig() error {
	var ms []mongo.WriteModel
	envs, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{})
	if err != nil {
		return err
	}

	resourceTypes := []models.ResourceType{
		models.ResourceTypePod,
		models.ResourceTypeDeployment,
		models.ResourceTypeReplicaSet,
		models.ResourceTypePVC,
		models.ResourceTypeService,
		models.ResourceTypeIngress,
		models.ResourceTypeStatefulSet,
		models.ResourceTypeCronJob,
		models.ResourceTypeHPA,
		models.ResourceTypePDB,
		models.ResourceTypeNetworkPolicy,
	}
	for _, env := range envs {
		if env.AnalysisConfig != nil {
			continue
		}

		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"product_name", env.ProductName}, {"env_name", env.EnvName}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"analysis_config.resource_types", resourceTypes},
					}},
				}),
		)

		log.Infof("add env %s", env.ProductName)

		if len(ms) >= 50 {
			log.Infof("update %d envs", len(ms))
			if _, err := mongodb.NewProductColl().BulkWrite(context.TODO(), ms); err != nil {
				return fmt.Errorf("udpate envs error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		log.Infof("update %d envs", len(ms))
		if _, err := mongodb.NewProductColl().BulkWrite(context.TODO(), ms); err != nil {
			return fmt.Errorf("udpate envs error: %s", err)
		}
	}
	return err
}

func migrateWorkflowV4ConcurrencyLimit() error {
	_, err := mongodb.NewWorkflowV4Coll().UpdateMany(context.Background(),
		bson.M{"multi_run": false}, bson.M{"$set": bson.M{"concurrency_limit": 1}})
	if err != nil {
		return errors.Wrap(err, "update workflow v4 concurrency limit 1")
	}
	_, err = mongodb.NewWorkflowV4Coll().UpdateMany(context.Background(),
		bson.M{"multi_run": true}, bson.M{"$set": bson.M{"concurrency_limit": -1}})
	if err != nil {
		return errors.Wrap(err, "update workflow v4 concurrency limit -1")
	}
	_, err = mongodb.NewWorkflowV4TemplateColl().UpdateMany(context.Background(),
		bson.M{"multi_run": false}, bson.M{"$set": bson.M{"concurrency_limit": 1}})
	if err != nil {
		return errors.Wrap(err, "update workflow v4 template concurrency limit 1")
	}
	_, err = mongodb.NewWorkflowV4TemplateColl().UpdateMany(context.Background(),
		bson.M{"multi_run": true}, bson.M{"$set": bson.M{"concurrency_limit": -1}})
	if err != nil {
		return errors.Wrap(err, "update workflow v4 template concurrency limit -1")
	}
	return nil
}

func migrateServiceModulesFieldForWorkflowV4Task() error {
	cursor, err := mongodb.NewworkflowTaskv4Coll().ListByCursor(&mongodb.ListWorkflowTaskV4Option{})
	if err != nil {
		return err
	}

	var mTasks []mongo.WriteModel
	change := false
	for cursor.Next(context.TODO()) {
		task := &models.WorkflowTask{}
		if err := cursor.Decode(task); err != nil {
			return err
		}
		task.WorkflowArgs, change, err = service.FillServiceModules2Jobs(task.WorkflowArgs)
		if err != nil {
			return err
		}
		if change {
			mTasks = append(mTasks,
				mongo.NewUpdateOneModel().
					SetFilter(bson.D{{"_id", task.ID}}).
					SetUpdate(bson.D{{"$set", bson.D{{"workflow_args", task.WorkflowArgs}}}}),
			)
			if len(mTasks) >= 50 {
				log.Infof("ua method:migrateServiceModulesFieldForWorkflowV4Task update %d workflowv4 tasks", len(mTasks))
				if _, err := mongodb.NewworkflowTaskv4Coll().BulkWrite(context.TODO(), mTasks); err != nil {
					return fmt.Errorf("udpate workflowV4 tasks error: %s", err)
				}
				mTasks = []mongo.WriteModel{}
			}
		}
	}
	if len(mTasks) > 0 {
		log.Infof("ua method:migrateServiceModulesFieldForWorkflowV4Task update %d workflowv4 tasks", len(mTasks))
		if _, err := mongodb.NewworkflowTaskv4Coll().BulkWrite(context.TODO(), mTasks); err != nil {
			return fmt.Errorf("udpate workflowV4 tasks error: %s", err)

		}
	}

	return nil
}

func migrateProjectNamePinyin() error {
	templateProducts, err := template.NewProductColl().List()
	if err != nil {
		return fmt.Errorf("list template product error: %s", err)
	}

	var ms []mongo.WriteModel
	for _, templateProduct := range templateProducts {
		if templateProduct.ProjectNamePinyin == "" {
			projectNamePinyin, projectNamePinyinFirstLetter := util.GetPinyinFromChinese(templateProduct.ProjectName)
			if projectNamePinyin != "" {
				ms = append(ms,
					mongo.NewUpdateOneModel().
						SetFilter(bson.D{{"product_name", templateProduct.ProductName}}).
						SetUpdate(bson.D{{"$set",
							bson.D{
								{"project_name_pinyin", projectNamePinyin},
								{"project_name_pinyin_first_letter", projectNamePinyinFirstLetter},
							}},
						}),
				)
				log.Infof("add productName %s", templateProduct.ProductName)
			}
		}
		if len(ms) >= 50 {
			log.Infof("update %d templateProduct", len(ms))
			if _, err := template.NewProductColl().BulkWrite(context.TODO(), ms); err != nil {
				return fmt.Errorf("update templateProduct error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		log.Infof("update %d templateProduct", len(ms))
		if _, err := template.NewProductColl().BulkWrite(context.TODO(), ms); err != nil {
			return fmt.Errorf("udpate workflowV4s error: %s", err)
		}
	}

	return nil
}

var newPackageDependenciesInV1180 = map[string][]string{
	"java": {"1.8.20", "1.9.0.1", "1.11.0.2"},
	"go":   {"1.20.7"},
	"node": {"12.22.12", "14.21.3", "16.20.2", "18.17.1"},
}

var oldPackageDependenciesInV1180 = map[string][]string{
	"dep":    {"0.4.1"},
	"yarn":   {"1.3.2"},
	"java":   {"1.6.6", "1.7.8", "1.9.0.1", "1.10.0.2"},
	"go":     {"1.8.3", "1.8.5", "1.9", "1.9.7", "1.10.1", "1.10.2", "1.11", "1.11.5", "1.12.1", "1.12.9"},
	"jMeter": {"3.2", "5.4.3"},
	"ginkgo": {"1.4.0", "1.6.0", "2.2.0", "2.3.1", "2.4.0"},
	"node":   {"6.11.2", "8.11.3"},
	"php":    {"5.5", "7.0", "7.1", "7.2"},
	"python": {"3.6.1", "3.7.0"},
}

func updatePackageDependencies() error {
	c := mongodb.NewInstallColl()
	installMap := systemservice.InitInstallMap()

	// update old package dependencies
	for name, versionList := range newPackageDependenciesInV1180 {
		for _, version := range versionList {
			fullName := fmt.Sprintf("%s-%s", name, version)
			installInfo, ok := installMap[fullName]
			if !ok {
				log.Infof("can't find %s install info from install map, skip", fullName)
				continue
			}
			pkgInfo := &models.Install{
				Name:         installInfo.Name,
				Version:      installInfo.Version,
				Scripts:      installInfo.Scripts,
				UpdateTime:   time.Now().Unix(),
				UpdateBy:     installInfo.UpdateBy,
				Envs:         installInfo.Envs,
				BinPath:      installInfo.BinPath,
				Enabled:      installInfo.Enabled,
				DownloadPath: installInfo.DownloadPath,
			}

			oid, err := primitive.ObjectIDFromHex(installInfo.ObjectIDHex)
			if err != nil {
				log.Errorf("failed to get %s ObjectID from hex, skip and err: %v", fullName, err)
				continue
			}
			query := bson.M{
				"_id":       oid,
				"update_by": setting.SystemUser,
			}
			change := bson.M{"$set": pkgInfo}

			result, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
			if err != nil {
				if !mongo.IsDuplicateKeyError(err) {
					return fmt.Errorf("update %s failed, err: %v", fullName, err)
				}
				log.Warnf("find %s has been existed, skip install", fullName)
				continue
			}
			if result.UpsertedCount > 0 {
				log.Infof("create %s install info success", fullName)
			}
		}
	}

	// delete old package dependencies
	for name, versionList := range oldPackageDependenciesInV1180 {
		for _, version := range versionList {
			query := bson.M{
				"name":      name,
				"version":   version,
				"update_by": setting.SystemUser,
			}
			_, err := c.DeleteOne(context.TODO(), query)
			if err != nil {
				return fmt.Errorf("failed to delete %s-%s package dependencies , err: %v", name, version, err)
			}
		}
	}
	return nil
}
