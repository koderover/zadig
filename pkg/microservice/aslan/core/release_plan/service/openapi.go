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

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type OpenAPIListReleasePlanResp struct {
	List  []*OpenAPIListReleasePlanInfo `json:"list"`
	Total int64                         `json:"total"`
}

type OpenAPIListReleasePlanInfo struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"       yaml:"-"                   json:"id"`
	Index       int64              `bson:"index"       yaml:"index"                   json:"index"`
	Name        string             `bson:"name"       yaml:"name"                   json:"name"`
	Manager     string             `bson:"manager"       yaml:"manager"                   json:"manager"`
	Description string             `bson:"description"       yaml:"description"                   json:"description"`
	CreatedBy   string             `bson:"created_by"       yaml:"created_by"                   json:"created_by"`
	CreateTime  int64              `bson:"create_time"       yaml:"create_time"                   json:"create_time"`
}

func OpenAPIListReleasePlans(pageNum, pageSize int64) (*OpenAPIListReleasePlanResp, error) {
	list, total, err := mongodb.NewReleasePlanColl().ListByOptions(&mongodb.ListReleasePlanOption{
		PageNum:        pageNum,
		PageSize:       pageSize,
		IsSort:         true,
		ExcludedFields: []string{"jobs", "logs"},
	})
	if err != nil {
		return nil, errors.Wrap(err, "ListReleasePlans")
	}
	resp := make([]*OpenAPIListReleasePlanInfo, 0)
	for _, plan := range list {
		resp = append(resp, &OpenAPIListReleasePlanInfo{
			ID:          plan.ID,
			Index:       plan.Index,
			Name:        plan.Name,
			Manager:     plan.Manager,
			Description: plan.Description,
			CreatedBy:   plan.CreatedBy,
			CreateTime:  plan.CreateTime,
		})
	}
	return &OpenAPIListReleasePlanResp{
		List:  resp,
		Total: total,
	}, nil
}

func OpenAPIGetReleasePlan(id string) (*models.ReleasePlan, error) {
	return mongodb.NewReleasePlanColl().GetByID(context.Background(), id)
}

type OpenAPICreateReleasePlanArgs struct {
	Name                string           `bson:"name"       yaml:"name"                   json:"name"`
	Manager             string           `bson:"manager"       yaml:"manager"                   json:"manager"`
	ManagerIdentityType string           `bson:"manager_identity_type"       yaml:"manager_identity_type"                   json:"manager_identity_type"`
	StartTime           int64            `bson:"start_time"       yaml:"start_time"                   json:"start_time"`
	EndTime             int64            `bson:"end_time"       yaml:"end_time"                   json:"end_time"`
	ScheduleExecuteTime int64            `bson:"schedule_execute_time"       yaml:"schedule_execute_time"                   json:"schedule_execute_time"`
	Description         string           `bson:"description"       yaml:"description"                   json:"description"`
	Approval            *models.Approval `bson:"approval"       yaml:"approval"                   json:"approval,omitempty"`
}

type OpenAPICreateReleasePlanResponse struct {
	ID string `bson:"id"       yaml:"id"                   json:"id"`
}

func OpenAPICreateReleasePlan(c *handler.Context, rawArgs *OpenAPICreateReleasePlanArgs) (*OpenAPICreateReleasePlanResponse, error) {
	args := &models.ReleasePlan{
		Name:                rawArgs.Name,
		Manager:             rawArgs.Manager,
		StartTime:           rawArgs.StartTime,
		EndTime:             rawArgs.EndTime,
		Description:         rawArgs.Description,
		ScheduleExecuteTime: rawArgs.ScheduleExecuteTime,
		Approval:            rawArgs.Approval,
	}
	if args.Name == "" || args.Manager == "" {
		return nil, errors.New("Required parameters are missing")
	}

	if err := lintReleaseTimeRange(args.StartTime, args.EndTime); err != nil {
		return nil, errors.Wrap(err, "lint release time range error")
	}
	if err := lintScheduleExecuteTime(args.ScheduleExecuteTime, args.StartTime, args.EndTime); err != nil {
		return nil, errors.Wrap(err, "lint schedule execute time error")
	}

	searchUserResp, err := user.New().SearchUser(&user.SearchUserArgs{
		Account:      args.Manager,
		IdentityType: rawArgs.ManagerIdentityType,
	})
	if err != nil {
		return nil, errors.Errorf("Failed to get user %s, error: %v", args.Manager, err)
	}
	if len(searchUserResp.Users) == 0 {
		return nil, errors.Errorf("User %s not found", args.Manager)
	}
	if len(searchUserResp.Users) > 1 {
		return nil, errors.Errorf("User %s search failed", args.Manager)
	}
	args.ManagerID = searchUserResp.Users[0].UID

	if args.Approval != nil {
		if err := lintApproval(args.Approval); err != nil {
			return nil, errors.Errorf("lintApproval error: %v", err)
		}
		if args.Approval.Type == config.LarkApproval || args.Approval.Type == config.LarkApprovalIntl {
			if err := createLarkApprovalDefinition(args.Approval.LarkApproval); err != nil {
				return nil, errors.Errorf("createLarkApprovalDefinition error: %v", err)
			}
		}
	}

	nextID, err := mongodb.NewCounterColl().GetNextSeq(setting.ReleasePlanFmt)
	if err != nil {
		log.Errorf("OpenAPICreateReleasePlan.GetNextSeq error: %v", err)
		return nil, e.ErrGetCounter.AddDesc(err.Error())
	}
	args.Index = nextID
	args.CreatedBy = c.UserName
	args.UpdatedBy = c.UserName
	args.CreateTime = time.Now().Unix()
	args.UpdateTime = time.Now().Unix()
	args.Status = config.ReleasePlanStatusPlanning

	planID, err := mongodb.NewReleasePlanColl().Create(args)
	if err != nil {
		return nil, errors.Wrap(err, "create release plan error")
	}

	go func() {
		if err := mongodb.NewReleasePlanLogColl().Create(&models.ReleasePlanLog{
			PlanID:     planID,
			Username:   c.UserName,
			Account:    c.Account,
			Verb:       VerbCreate,
			TargetName: args.Name,
			TargetType: TargetTypeReleasePlan,
			CreatedAt:  time.Now().Unix(),
		}); err != nil {
			log.Errorf("create release plan log error: %v", err)
		}
	}()

	return &OpenAPICreateReleasePlanResponse{
		ID: planID,
	}, nil
}

type OpenAPIUpdateReleasePlanWithJobsArgs struct {
	Name                string                   `bson:"name"                    yaml:"name"                    json:"name"`
	Manager             string                   `bson:"manager"                 yaml:"manager"                 json:"manager"`
	ManagerIdentityType string                   `bson:"manager_identity_type"   yaml:"manager_identity_type"   json:"manager_identity_type"`
	StartTime           int64                    `bson:"start_time"              yaml:"start_time"              json:"start_time"`
	EndTime             int64                    `bson:"end_time"                yaml:"end_time"                json:"end_time"`
	ScheduleExecuteTime int64                    `bson:"schedule_execute_time"   yaml:"schedule_execute_time"   json:"schedule_execute_time"`
	Description         string                   `bson:"description"             yaml:"description"             json:"description"`
	Approval            *models.Approval         `bson:"approval"                yaml:"approval"                json:"approval,omitempty"`
	Jobs                []*OpenAPIReleasePlanJob `bson:"jobs"                    yaml:"jobs"                    json:"jobs"`
}

type OpenAPIReleasePlanJob struct {
	Name string                    `bson:"name"       yaml:"name"                   json:"name"`
	Type config.ReleasePlanJobType `bson:"type"       yaml:"type"                   json:"type"`
	Spec interface{}               `bson:"spec"       yaml:"spec"                   json:"spec"`
}

type OpenAPITextReleaseJobSpec struct {
	Content string `bson:"content"      yaml:"content"     json:"content"`
	Remark  string `bson:"remark"       yaml:"remark"      json:"remark"`
}

type OpenAPIWorkflowReleaseJobSpec struct {
	WorkflowKey string                                      `bson:"workflow_key"        yaml:"workflow_key"                    json:"workflow_key"`
	ProjectKey  string                                      `bson:"project_key"         yaml:"project_key"                     json:"project_key"`
	Inputs      []*workflowservice.CreateCustomTaskJobInput `bson:"inputs"              yaml:"inputs"                          json:"inputs"`
	Remark      string                                      `bson:"remark"              yaml:"remark"                          json:"remark"`
}

func OpenAPICreateReleasePlanWithJobs(c *handler.Context, id string, rawArgs *OpenAPIUpdateReleasePlanWithJobsArgs) error {
	plan, err := mongodb.NewReleasePlanColl().GetByID(context.Background(), id)
	if err != nil {
		return errors.Wrap(err, "get release plan error")
	}

	if rawArgs.Name == "" || rawArgs.Manager == "" {
		return errors.New("Required parameters are missing")
	}

	if plan.Status == config.ReleasePlanStatusSuccess || plan.Status == config.ReleasePlanStatusExecuting || plan.Status == config.ReleasePlanStatusWaitForApprove {
		return errors.New("release plan status is success, executing or wait for approve, can't update")
	}

	plan.Name = rawArgs.Name
	plan.Manager = rawArgs.Manager
	plan.StartTime = rawArgs.StartTime
	plan.EndTime = rawArgs.EndTime
	plan.ScheduleExecuteTime = rawArgs.ScheduleExecuteTime
	plan.Description = rawArgs.Description
	plan.Approval = rawArgs.Approval

	if err := lintReleaseTimeRange(plan.StartTime, plan.EndTime); err != nil {
		return errors.Wrap(err, "lint release time range error")
	}
	if err := lintScheduleExecuteTime(plan.ScheduleExecuteTime, plan.StartTime, plan.EndTime); err != nil {
		return errors.Wrap(err, "lint schedule execute time error")
	}

	searchUserResp, err := user.New().SearchUser(&user.SearchUserArgs{
		Account:      plan.Manager,
		IdentityType: rawArgs.ManagerIdentityType,
	})
	if err != nil {
		return errors.Errorf("Failed to get user %s, error: %v", plan.Manager, err)
	}
	if len(searchUserResp.Users) == 0 {
		return errors.Errorf("User %s not found", plan.Manager)
	}
	if len(searchUserResp.Users) > 1 {
		return errors.Errorf("User %s search failed", plan.Manager)
	}
	plan.ManagerID = searchUserResp.Users[0].UID

	if plan.Approval != nil {
		if err := lintApproval(plan.Approval); err != nil {
			return errors.Errorf("lintApproval error: %v", err)
		}
		if plan.Approval.Type == config.LarkApproval || plan.Approval.Type == config.LarkApprovalIntl {
			if err := createLarkApprovalDefinition(plan.Approval.LarkApproval); err != nil {
				return errors.Errorf("createLarkApprovalDefinition error: %v", err)
			}
		}
	}

	plan.UpdatedBy = c.UserName
	plan.UpdateTime = time.Now().Unix()
	plan.Status = config.ReleasePlanStatusPlanning

	newJobs := make([]*models.ReleaseJob, 0)
	for _, job := range rawArgs.Jobs {
		switch job.Type {
		case config.JobText:
			textSpec := &OpenAPITextReleaseJobSpec{}
			err = models.IToi(job.Spec, textSpec)
			if err != nil {
				return errors.Wrap(err, "invalid spec")
			}

			newJobs = append(newJobs, &models.ReleaseJob{
				ID:   uuid.New().String(),
				Name: job.Name,
				Type: job.Type,
				Spec: models.TextReleaseJobSpec{
					Content: textSpec.Content,
					Remark:  textSpec.Remark,
				},
				ReleaseJobRuntime: models.ReleaseJobRuntime{},
			})
		case config.JobWorkflow:
			workflowSpec := &OpenAPIWorkflowReleaseJobSpec{}
			err = models.IToi(job.Spec, workflowSpec)
			if err != nil {
				return errors.Wrap(err, "invalid spec")
			}

			workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowSpec.WorkflowKey)
			if err != nil {
				log.Errorf("cannot find workflow %s, the error is: %v", workflowSpec.WorkflowKey, err)
				return e.ErrFindWorkflow.AddDesc(err.Error())
			}
			workflow.Remark = workflowSpec.Remark

			workflowController := controller.CreateWorkflowController(workflow)
			if err := workflowController.SetPreset(nil); err != nil {
				log.Errorf("cannot get workflow %s preset, the error is: %v", workflowSpec.WorkflowKey, err)
				return e.ErrPresetWorkflow.AddDesc(err.Error())
			}

			inputMap := make(map[string]interface{})
			for _, input := range workflowSpec.Inputs {
				inputMap[input.JobName] = input.Parameters
			}

			for _, stage := range workflow.Stages {
				jobList := make([]*commonmodels.Job, 0)
				for _, job := range stage.Jobs {
					// if a job is found, add it to the job creation list
					if inputParam, ok := inputMap[job.Name]; ok {
						updater, err := workflowservice.GetInputUpdater(job, inputParam, workflow)
						if err != nil {
							return err
						}
						newJob, err := updater.UpdateJobSpec(job)
						if err != nil {
							log.Errorf("Failed to update jobspec for job: %s, error: %s", job.Name, err)
							return fmt.Errorf("failed to update jobspec for job: %s, err: %w", job.Name, err)
						}
						job.Skipped = false
						jobList = append(jobList, newJob)
					} else {
						job.Skipped = true
						jobList = append(jobList, job)
					}
				}
				stage.Jobs = jobList
			}

			newJobs = append(newJobs, &models.ReleaseJob{
				ID:   uuid.New().String(),
				Name: job.Name,
				Type: job.Type,
				Spec: models.WorkflowReleaseJobSpec{
					Workflow: workflow,
				},
				ReleaseJobRuntime: models.ReleaseJobRuntime{},
			})
		}
	}

	plan.Jobs = newJobs

	err = mongodb.NewReleasePlanColl().UpdateByID(c, id, plan)
	if err != nil {
		return errors.Wrap(err, "update release plan error")
	}

	go func() {
		if err := mongodb.NewReleasePlanLogColl().Create(&models.ReleasePlanLog{
			PlanID:     plan.ID.Hex(),
			Username:   c.UserName,
			Account:    c.Account,
			Verb:       VerbUpdate,
			TargetName: plan.Name,
			TargetType: TargetTypeReleasePlan,
			CreatedAt:  time.Now().Unix(),
		}); err != nil {
			log.Errorf("create release plan log error: %v", err)
		}
	}()

	return nil
}
