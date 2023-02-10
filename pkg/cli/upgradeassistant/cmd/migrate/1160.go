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
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	projectservice "github.com/koderover/zadig/pkg/microservice/aslan/core/project/service"
	systemservice "github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	steptypes "github.com/koderover/zadig/pkg/types/step"
)

func init() {
	upgradepath.RegisterHandler("1.15.0", "1.16.0", V1150ToV1160)
	upgradepath.RegisterHandler("1.16.0", "1.15.0", V1160ToV1150)
}

func V1150ToV1160() error {
	if err := migrateReleaseCenter(); err != nil {
		log.Errorf("migrateReleaseCenter err:%s", err)
		return err
	}
	if err := initWorkflowV4TaskStats(); err != nil {
		log.Errorf("initWorkflowV4TaskStats err:%s", err)
		return err
	}
	if err := addDisplayNameToWorkflowV4(); err != nil {
		log.Errorf("addDisplayNameToWorkflowV4 err:%s", err)
		return err
	}
	if err := addDisplayNameToWorkflow(); err != nil {
		log.Errorf("addDisplayNameToWorkflow err:%s", err)
		return err
	}
	if err := createNewPackageDependencies(); err != nil {
		log.Errorf("createNewPackageDependencies err:%s", err)
		return err
	}

	if err := updateWorkflowApproval(); err != nil {
		log.Errorf("updateWorkflowApproval err:%s", err)
		return err
	}
	if err := updateWorkflowTaskApproval(); err != nil {
		log.Errorf("updateWorkflowTaskApproval err:%s", err)
		return err
	}
	if err := updateCronjobApproval(); err != nil {
		log.Errorf("updateCronjobApproval err:%s", err)
		return err
	}
	if err := updateWorkflowTemplateApproval(); err != nil {
		log.Errorf("updateWorkflowTemplateApproval err:%s", err)
		return err
	}

	if err := HandleK8sYamlVars(); err != nil {
		log.Errorf("HandleK8sYamlVars err:%s", err)
	}

	return nil
}

func V1160ToV1150() error {
	return nil
}

func initWorkflowV4TaskStats() error {
	taskCursor, err := mongodb.NewworkflowTaskv4Coll().ListByCursor(&mongodb.ListWorkflowTaskV4Option{})
	if err != nil {
		return err
	}
	statMap := map[string]*models.WorkflowStat{}
	for taskCursor.Next(context.Background()) {
		var workflowTask models.WorkflowTask
		if err := taskCursor.Decode(&workflowTask); err != nil {
			return err
		}
		if workflowTask.Status != config.StatusPassed && workflowTask.Status != config.StatusFailed && workflowTask.Status != config.StatusTimeout {
			continue
		}
		totalSuccess := 0
		totalFailure := 0
		if workflowTask.Status == config.StatusPassed {
			totalSuccess = 1
			totalFailure = 0
		} else {
			totalSuccess = 0
			totalFailure = 1
		}
		duration := workflowTask.EndTime - workflowTask.StartTime
		if stat, exist := statMap[workflowTask.WorkflowName]; !exist {
			statMap[workflowTask.WorkflowName] = &models.WorkflowStat{
				ProductName:   workflowTask.ProjectName,
				Name:          workflowTask.WorkflowName,
				Type:          string(config.WorkflowTypeV4),
				TotalDuration: duration,
				TotalSuccess:  totalSuccess,
				TotalFailure:  totalFailure,
				CreatedAt:     time.Now().Unix(),
				UpdatedAt:     time.Now().Unix(),
			}
		} else {
			stat.TotalDuration += duration
			stat.TotalSuccess += totalSuccess
			stat.TotalFailure += totalFailure
		}
	}
	var ms []mongo.WriteModel
	for _, stat := range statMap {
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"name", stat.Name}, {"type", stat.Type}}).
				SetUpdate(bson.D{{"$set", stat}}).SetUpsert(true),
		)
	}
	if len(ms) > 0 {
		if _, err := mongodb.NewWorkflowStatColl().BulkWrite(context.TODO(), ms); err != nil {
			return fmt.Errorf("udpate workflowV4s stat error: %s", err)
		}
	}
	return nil
}

func addDisplayNameToWorkflowV4() error {
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
		displayName := workflow.DisplayName
		if workflow.DisplayName == "" {
			displayName = workflow.Name
		}
		for _, webhook := range workflow.HookCtls {
			setPRsforWorkflowV4(webhook.WorkflowArg)
		}
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflow.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"display_name", displayName},
						{"hook_ctl", workflow.HookCtls},
					}},
				}),
		)
		if len(ms) >= 50 {
			log.Infof("update %d workflowv4s", len(ms))
			if _, err := mongodb.NewWorkflowV4Coll().BulkWrite(context.TODO(), ms); err != nil {
				return fmt.Errorf("udpate workflowV4s error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		log.Infof("update %d workflowv4s", len(ms))
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
		displayName := workflowTask.WorkflowDisplayName
		if workflowTask.WorkflowDisplayName == "" {
			displayName = workflowTask.WorkflowName
		}
		setPRsforWorkflowV4(workflowTask.OriginWorkflowArgs)
		mTasks = append(mTasks,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflowTask.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"workflow_display_name", displayName},
						{"origin_workflow_args", workflowTask.OriginWorkflowArgs},
					}},
				}),
		)
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

func setPRsforWorkflowV4(workflow *models.WorkflowV4) {
	if workflow == nil {
		return
	}
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			switch job.JobType {
			case config.JobZadigBuild:
				spec := &models.ZadigBuildJobSpec{}
				if err := models.IToi(job.Spec, spec); err != nil {
					continue
				}
				for _, svc := range spec.ServiceAndBuilds {
					for _, repo := range svc.Repos {
						setPRs(repo)
					}
				}
				job.Spec = spec
			case config.JobFreestyle:
				spec := &models.FreestyleJobSpec{}
				if err := models.IToi(job.Spec, spec); err != nil {
					continue
				}
				for _, step := range spec.Steps {
					if step.StepType != config.StepGit {
						continue
					}
					stepSpec := &steptypes.StepGitSpec{}
					if err := models.IToi(step.Spec, stepSpec); err != nil {
						continue
					}
					for _, repo := range stepSpec.Repos {
						setPRs(repo)
					}
					step.Spec = stepSpec
				}
				job.Spec = spec
			case config.JobZadigTesting:
				spec := &models.ZadigTestingJobSpec{}
				if err := models.IToi(job.Spec, spec); err != nil {
					continue
				}
				for _, test := range spec.TestModules {
					for _, repo := range test.Repos {
						setPRs(repo)
					}
				}
				job.Spec = spec
			}
		}
	}
}

func addDisplayNameToWorkflow() error {
	cursor, err := mongodb.NewWorkflowColl().ListByCursor(&mongodb.ListWorkflowOption{})
	if err != nil {
		return err
	}
	var ms []mongo.WriteModel
	for cursor.Next(context.Background()) {
		var workflow models.Workflow
		if err := cursor.Decode(&workflow); err != nil {
			return err
		}
		if workflow.DisplayName != "" {
			continue
		}
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflow.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"display_name", workflow.Name},
					}},
				}),
		)
		if len(ms) >= 50 {
			log.Infof("update %d workflows", len(ms))
			if _, err := mongodb.NewWorkflowColl().BulkWrite(context.TODO(), ms); err != nil {
				return fmt.Errorf("udpate workflows error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		log.Infof("update %d workflows", len(ms))
		if _, err := mongodb.NewWorkflowColl().BulkWrite(context.TODO(), ms); err != nil {
			return fmt.Errorf("udpate workflows error: %s", err)
		}
	}

	taskCursor, err := mongodb.NewTaskColl().ListByCursor(&mongodb.ListAllTaskOption{})
	if err != nil {
		return err
	}
	var mTasks []mongo.WriteModel
	for taskCursor.Next(context.Background()) {
		var workflowTask task.Task
		if err := cursor.Decode(&workflowTask); err != nil {
			return err
		}
		displayName := workflowTask.PipelineDisplayName
		if workflowTask.PipelineDisplayName == "" {
			displayName = workflowTask.PipelineName
		}
		setPRsForWorkflowTask(&workflowTask)
		mTasks = append(mTasks,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflowTask.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"pipeline_display_name", displayName},
						{"workflow_args", workflowTask.WorkflowArgs},
					}},
				}),
		)
		if len(mTasks) >= 50 {
			log.Infof("update %d workflow tasks", len(mTasks))
			if _, err := mongodb.NewTaskColl().BulkWrite(context.TODO(), mTasks); err != nil {
				return fmt.Errorf("udpate workflow tasks error: %s", err)
			}
			mTasks = []mongo.WriteModel{}
		}
	}
	if len(mTasks) > 0 {
		log.Infof("update %d workflow tasks", len(mTasks))
		if _, err := mongodb.NewTaskColl().BulkWrite(context.TODO(), mTasks); err != nil {
			return fmt.Errorf("udpate workflow tasks error: %s", err)
		}
	}
	return nil
}

var newPackageDependenciesInV1160 = map[string][]string{
	"dep":    {"0.5.3", "0.5.4"},
	"ginkgo": {"2.2.0", "2.3.1", "2.4.0", "2.5.0"},
	"glide":  {"0.13.3"},
	"go":     {"1.18.8", "1.19.3"},
	"jMeter": {"5.4.3", "5.5"},
	"java":   {"17", "19"},
	"maven":  {"3.8.6"},
	"node":   {"16.18.1", "18.12.1"},
	"php":    {"8.0.25", "8.1.12"},
	"python": {"3.10.8", "3.11"},
	"yarn":   {"3.2.0", "3.2.4"},
}

func createNewPackageDependencies() error {
	c := mongodb.NewInstallColl()
	installMap := systemservice.InitInstallMap()

	for name, versionList := range newPackageDependenciesInV1160 {
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
			query := bson.M{"_id": oid}
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
	return nil
}

func setPRsForWorkflowTask(t *task.Task) {
	if t.WorkflowArgs == nil {
		return
	}
	for _, target := range t.WorkflowArgs.Target {
		if target.Build == nil {
			continue
		}
		for _, repo := range target.Build.Repos {
			setPRs(repo)
		}
	}
	for _, test := range t.WorkflowArgs.Tests {
		for _, repo := range test.Builds {
			setPRs(repo)
		}
	}
}

func setPRs(repo *types.Repository) {
	if len(repo.PRs) > 0 {
		return
	}
	if repo.PR > 0 {
		repo.PRs = []int{repo.PR}
	}
}

func migrateReleaseCenter() error {
	logger := log.SugaredLogger()
	_, err := templaterepo.NewProductColl().Find("release-center")
	if err == nil {
		logger.Info("project release-center already exists")
		return nil
	}
	workflows, _, err := mongodb.NewWorkflowV4Coll().List(&mongodb.ListWorkflowV4Option{ProjectName: setting.EnterpriseProject}, 0, 0)
	if err != nil {
		return err
	}
	if len(workflows) <= 0 {
		logger.Infof("no workflow found in project %s", setting.EnterpriseProject)
		return nil
	}
	product := &template.Product{
		ProjectName: "发布中心",
		ProductName: "release-center",
		Description: "migrate from deploy center",
		Enabled:     true,
		ProductFeature: &template.ProductFeature{
			BasicFacility: "kubernetes",
			CreateEnvType: "system",
			DeployType:    "k8s",
		},
		Public: false,
	}
	clusters, err := mongodb.NewK8SClusterColl().List(&mongodb.ClusterListOpts{})
	if err != nil {
		return err
	}
	for _, cluster := range clusters {
		product.ClusterIDs = append(product.ClusterIDs, cluster.ID.Hex())
	}
	if err := projectservice.CreateProductTemplate(product, logger); err != nil {
		return err
	}
	if err := projectservice.UpdateProductTmplStatus(product.ProductName, "0", logger); err != nil {
		return err
	}
	var mWorkflow []mongo.WriteModel
	for _, workflow := range workflows {
		mWorkflow = append(mWorkflow,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflow.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"project", "release-center"},
						{"category", setting.ReleaseWorkflow},
					}},
				}),
		)
	}
	if len(mWorkflow) > 0 {
		log.Infof("migrate %d workflows", len(mWorkflow))
		if _, err := mongodb.NewWorkflowV4Coll().BulkWrite(context.TODO(), mWorkflow); err != nil {
			return fmt.Errorf("migrate workflow error: %s", err)
		}
	}
	return nil
}

// WorkflowV4V1150 is part of the older version of WorkflowV4, which used to update data
type WorkflowV4V1150 struct {
	ID     primitive.ObjectID    `bson:"_id,omitempty"       yaml:"-"            json:"id"`
	Stages []*WorkflowStageV1150 `bson:"stages"              yaml:"stages"       json:"stages"`
}

type WorkflowV4TemplateV1150 struct {
	ID     primitive.ObjectID    `bson:"_id,omitempty"       yaml:"id"                  json:"id"`
	Stages []*WorkflowStageV1150 `bson:"stages"              yaml:"stages"             json:"stages"`
}

type WorkflowTaskV1150 struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty"       yaml:"id"                  json:"id"`
	OriginWorkflowArgs *WorkflowV4V1150   `bson:"origin_workflow_args"      json:"origin_workflow_args"`
}

type CronjobV1150 struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"                       json:"id"`
	WorkflowV4Args *WorkflowV4V1150   `bson:"workflow_v4_args"                    json:"workflow_v4_args"`
}

type WorkflowStageV1150 struct {
	Name          string         `bson:"name"          yaml:"name"         json:"name"`
	Parallel      bool           `bson:"parallel"      yaml:"parallel"     json:"parallel"`
	ApprovalV1150 *ApprovalV1150 `bson:"approval"      yaml:"approval"     json:"approval"`
	Jobs          []*models.Job  `bson:"jobs"          yaml:"jobs"         json:"jobs"`
}

type WorkflowStageV1160CompatibleV1150 struct {
	Name     string                        `bson:"name"          yaml:"name"         json:"name"`
	Parallel bool                          `bson:"parallel"      yaml:"parallel"     json:"parallel"`
	Approval *ApprovalV1160CompatibleV1150 `bson:"approval"      yaml:"approval"     json:"approval"`
	Jobs     []*models.Job                 `bson:"jobs"          yaml:"jobs"         json:"jobs"`
}

type ApprovalV1150 struct {
	// Type is the new field in 1.16 approval struct, which used to check whether the data is before 1.16
	Type            config.ApprovalType    `bson:"type,omitempty"              yaml:"type"                       json:"type"`
	Enabled         bool                   `bson:"enabled"                     yaml:"enabled"                    json:"enabled"`
	ApproveUsers    []*models.User         `bson:"approve_users"               yaml:"approve_users"              json:"approve_users"`
	Timeout         int                    `bson:"timeout"                     yaml:"timeout"                    json:"timeout"`
	NeededApprovers int                    `bson:"needed_approvers"            yaml:"needed_approvers"           json:"needed_approvers"`
	Description     string                 `bson:"description"                 yaml:"description"                json:"description"`
	RejectOrApprove config.ApproveOrReject `bson:"reject_or_approve"           yaml:"-"                          json:"reject_or_approve"`
}

// ApprovalV1160CompatibleV1150 is the V1160 approval struct with V1150 fields
type ApprovalV1160CompatibleV1150 struct {
	*ApprovalV1150 `json:",inline" bson:",inline"`
	NativeApproval *models.NativeApproval `bson:"native_approval"             yaml:"native_approval,omitempty"     json:"native_approval,omitempty"`
	LarkApproval   *models.LarkApproval   `bson:"lark_approval"               yaml:"lark_approval,omitempty"       json:"lark_approval,omitempty"`
}

func updateWorkflowApproval() error {
	coll := mongodb.NewWorkflowV4Coll()
	cursor, err := coll.Collection.Find(context.TODO(), bson.M{})
	if err != nil {
		return err
	}

	var ms []mongo.WriteModel
	for cursor.Next(context.Background()) {
		var workflow WorkflowV4V1150
		if err := cursor.Decode(&workflow); err != nil {
			return err
		}
		if workflow.Stages == nil {
			continue
		}
		newStages := UpdateStages(workflow.Stages)
		if newStages == nil {
			continue
		}
		ms = append(ms, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": workflow.ID}).
			SetUpdate(bson.M{"$set": bson.M{"stages": newStages}}).SetUpsert(false),
		)
		if len(ms) >= 100 {
			if re, err := coll.BulkWrite(context.TODO(), ms); err != nil {
				return errors.Wrap(err, "bulk write")
			} else {
				log.Infof("updateWorkflowApproval ModifiedNum: %d UpsertNum: %d", re.ModifiedCount, re.UpsertedCount)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		if re, err := coll.BulkWrite(context.TODO(), ms); err != nil {
			return errors.Wrap(err, "bulk write")
		} else {
			log.Infof("updateWorkflowApproval ModifiedNum: %d UpsertNum: %d", re.ModifiedCount, re.UpsertedCount)
		}
	}
	return nil
}

func updateWorkflowTemplateApproval() error {
	coll := mongodb.NewWorkflowV4TemplateColl()
	cursor, err := coll.Collection.Find(context.TODO(), bson.M{})
	if err != nil {
		return err
	}

	var ms []mongo.WriteModel
	for cursor.Next(context.Background()) {
		var tpl WorkflowV4TemplateV1150
		if err := cursor.Decode(&tpl); err != nil {
			return err
		}
		if tpl.Stages == nil {
			continue
		}
		newStages := UpdateStages(tpl.Stages)
		if newStages == nil {
			continue
		}
		ms = append(ms, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": tpl.ID}).
			SetUpdate(bson.M{"$set": bson.M{"stages": newStages}}).SetUpsert(false),
		)
		if len(ms) >= 100 {
			if re, err := coll.BulkWrite(context.TODO(), ms); err != nil {
				return errors.Wrap(err, "bulk write")
			} else {
				log.Infof("updateWorkflowTemplateApproval ModifiedNum: %d UpsertNum: %d", re.ModifiedCount, re.UpsertedCount)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		if re, err := coll.BulkWrite(context.TODO(), ms); err != nil {
			return errors.Wrap(err, "bulk write")
		} else {
			log.Infof("updateWorkflowTemplateApproval ModifiedNum: %d UpsertNum: %d", re.ModifiedCount, re.UpsertedCount)
		}
	}
	return nil
}

func updateWorkflowTaskApproval() error {
	coll := mongodb.NewworkflowTaskv4Coll()
	cursor, err := coll.Collection.Find(context.TODO(), bson.M{})
	if err != nil {
		return err
	}

	var ms []mongo.WriteModel
	for cursor.Next(context.Background()) {
		var task WorkflowTaskV1150
		if err := cursor.Decode(&task); err != nil {
			return err
		}
		if task.OriginWorkflowArgs == nil || task.OriginWorkflowArgs.Stages == nil {
			continue
		}
		newStages := UpdateStages(task.OriginWorkflowArgs.Stages)
		if newStages == nil {
			continue
		}
		ms = append(ms, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": task.ID}).
			SetUpdate(bson.M{"$set": bson.M{"origin_workflow_args.stages": newStages}}).SetUpsert(false),
		)
		if len(ms) >= 100 {
			if re, err := coll.BulkWrite(context.TODO(), ms); err != nil {
				return errors.Wrap(err, "bulk write")
			} else {
				log.Infof("updateWorkflowTaskApproval ModifiedNum: %d UpsertNum: %d", re.ModifiedCount, re.UpsertedCount)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		if re, err := coll.BulkWrite(context.TODO(), ms); err != nil {
			return errors.Wrap(err, "bulk write")
		} else {
			log.Infof("updateWorkflowTaskApproval ModifiedNum: %d UpsertNum: %d", re.ModifiedCount, re.UpsertedCount)
		}
	}
	return nil
}

func updateCronjobApproval() error {
	coll := mongodb.NewCronjobColl()
	cursor, err := coll.Collection.Find(context.TODO(), bson.M{})
	if err != nil {
		return err
	}

	var ms []mongo.WriteModel
	for cursor.Next(context.Background()) {
		var cron CronjobV1150
		if err := cursor.Decode(&cron); err != nil {
			return err
		}
		if cron.WorkflowV4Args == nil || cron.WorkflowV4Args.Stages == nil {
			continue
		}
		newStages := UpdateStages(cron.WorkflowV4Args.Stages)
		if newStages == nil {
			continue
		}
		ms = append(ms, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": cron.ID}).
			SetUpdate(bson.M{"$set": bson.M{"workflow_v4_args.stages": newStages}}).SetUpsert(false),
		)
		if len(ms) >= 100 {
			if re, err := coll.BulkWrite(context.TODO(), ms); err != nil {
				return errors.Wrap(err, "bulk write")
			} else {
				log.Infof("updateWorkflowCronjobApproval ModifiedNum: %d UpsertNum: %d", re.ModifiedCount, re.UpsertedCount)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		if re, err := coll.BulkWrite(context.TODO(), ms); err != nil {
			return errors.Wrap(err, "bulk write")
		} else {
			log.Infof("updateWorkflowCronjobApproval ModifiedNum: %d UpsertNum: %d", re.ModifiedCount, re.UpsertedCount)
		}
	}
	return nil
}

func UpdateStages(list []*WorkflowStageV1150) []*WorkflowStageV1160CompatibleV1150 {
	var newStages []*WorkflowStageV1160CompatibleV1150
	for _, stage := range list {
		// If type field exists, the workflow data is not earlier than V1160, skip
		if stage.ApprovalV1150 != nil && stage.ApprovalV1150.Type != "" {
			return nil
		}

		var updateApprove *ApprovalV1160CompatibleV1150
		if stage.ApprovalV1150 != nil {
			updateApprove = &ApprovalV1160CompatibleV1150{
				ApprovalV1150: stage.ApprovalV1150,
				NativeApproval: &models.NativeApproval{
					Timeout:         stage.ApprovalV1150.Timeout,
					ApproveUsers:    stage.ApprovalV1150.ApproveUsers,
					NeededApprovers: stage.ApprovalV1150.NeededApprovers,
					RejectOrApprove: stage.ApprovalV1150.RejectOrApprove,
				},
			}
			updateApprove.Type = config.NativeApproval
		}
		newStages = append(newStages, &WorkflowStageV1160CompatibleV1150{
			Name:     stage.Name,
			Parallel: stage.Parallel,
			Jobs:     stage.Jobs,
			Approval: updateApprove,
		})
	}
	return newStages
}
