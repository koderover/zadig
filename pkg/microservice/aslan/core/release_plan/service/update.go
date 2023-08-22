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
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/orm"
)

const (
	VerbUpdateName      = "update_name"
	VerbUpdateDesc      = "update_description"
	VerbUpdateTimeRange = "update_time_range"
	VerbUpdateManager   = "update_manager"

	VerbCreateReleaseJob = "create_release_job"
	VerbUpdateReleaseJob = "update_release_job"
	VerbDeleteReleaseJob = "delete_release_job"

	VerbUpdateApproval = "update_approval"
)

type PlanUpdater interface {
	// Update returns the old data and the updated data
	Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error)
	Lint() error
}

func NewPlanUpdater(args *UpdateReleasePlanArgs) (PlanUpdater, error) {
	switch args.Verb {
	case VerbUpdateName:
		return NewNameUpdater(args)
	case VerbUpdateDesc:
		return NewDescUpdater(args)
	case VerbUpdateTimeRange:
		return NewTimeRangeUpdater(args)
	case VerbUpdateManager:
		return NewManagerUpdater(args)
	case VerbCreateReleaseJob:
		return NewCreateReleaseJobUpdater(args)
	case VerbUpdateReleaseJob:
		return NewUpdateReleaseJobUpdater(args)
	case VerbDeleteReleaseJob:
		return NewDeleteReleaseJobUpdater(args)
	case VerbUpdateApproval:
		return NewUpdateApprovalUpdater(args)
	default:
		return nil, fmt.Errorf("invalid verb: %s", args.Verb)
	}
}

type NameUpdater struct {
	Name string `json:"name"`
}

func NewNameUpdater(args *UpdateReleasePlanArgs) (*NameUpdater, error) {
	var updater NameUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *NameUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	before, after = plan.Name, u.Name
	plan.Name = u.Name
	return
}

func (u *NameUpdater) Lint() error {
	if u.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	return nil
}

type DescUpdater struct {
	Description string `json:"description"`
}

func NewDescUpdater(args *UpdateReleasePlanArgs) (*DescUpdater, error) {
	var updater DescUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *DescUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	before, after = plan.Description, u.Description
	plan.Description = u.Description
	return
}

func (u *DescUpdater) Lint() error {
	return nil
}

type TimeRangeUpdater struct {
	StartTime int64 `json:"start_time"`
	EndTime   int64 `json:"end_time"`
}

func NewTimeRangeUpdater(args *UpdateReleasePlanArgs) (*TimeRangeUpdater, error) {
	var updater TimeRangeUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *TimeRangeUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {

	before, after = fmt.Sprintf("%s-%s", time.Unix(plan.StartTime, 0).Format("2006-01-02 15:04:05"), plan.EndTime), fmt.Sprintf("%d-%d", u.StartTime, u.EndTime
	plan.StartTime = u.StartTime
	plan.EndTime = u.EndTime
	return nil
}

func (u *TimeRangeUpdater) Lint() error {
	if u.StartTime == 0 || u.EndTime == 0 {
		return fmt.Errorf("start_time and end_time cannot be empty")
	}
	if u.StartTime >= u.EndTime {
		return fmt.Errorf("start_time must be less than end_time")
	}
	if u.EndTime < time.Now().Unix() {
		return fmt.Errorf("end_time must be greater than now")
	}
	return nil
}

type ManagerUpdater struct {
	ManagerID string `json:"manager_id"`
	Name      string `json:"name"`
}

func NewManagerUpdater(args *UpdateReleasePlanArgs) (*ManagerUpdater, error) {
	var updater ManagerUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *ManagerUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	plan.ManagerID = u.ManagerID
	plan.Manager = u.Name
	return nil
}

func (u *ManagerUpdater) Lint() error {
	if u.ManagerID == "" {
		return fmt.Errorf("manager_id cannot be empty")
	}
	user, err := orm.GetUserByUid(u.ManagerID, repository.DB)
	if err != nil || user == nil {
		return fmt.Errorf("user not found")
	}
	if u.Name != user.Name {
		return fmt.Errorf("name not match")
	}
	return nil
}

type CreateReleaseJobUpdater struct {
	Name string      `json:"name"`
	Type string      `json:"type"`
	Spec interface{} `json:"spec"`
}

func NewCreateReleaseJobUpdater(args *UpdateReleasePlanArgs) (*CreateReleaseJobUpdater, error) {
	var updater CreateReleaseJobUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *CreateReleaseJobUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	job := &models.ReleaseJob{
		ID:   uuid.New().String(),
		Name: u.Name,
		Type: u.Type,
		Spec: u.Spec,
	}
	plan.Jobs = append(plan.Jobs, job)
	return nil
}

func (u *CreateReleaseJobUpdater) Lint() error {
	if u.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	return lintReleaseJob(u.Type, u.Spec)
}

type UpdateReleaseJobUpdater struct {
	ID   string      `json:"id"`
	Name string      `json:"name"`
	Type string      `json:"type"`
	Spec interface{} `json:"spec"`
}

func NewUpdateReleaseJobUpdater(args *UpdateReleasePlanArgs) (*UpdateReleaseJobUpdater, error) {
	var updater UpdateReleaseJobUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *UpdateReleaseJobUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	for _, job := range plan.Jobs {
		if job.ID == u.ID {
			if job.Type != u.Type {
				return fmt.Errorf("job type cannot be changed")
			}
			job.Name = u.Name
			job.Spec = u.Spec
			return nil
		}
	}
	return fmt.Errorf("job %s-%s not found", u.Name, u.ID)
}

func (u *UpdateReleaseJobUpdater) Lint() error {
	if u.ID == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if u.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	return lintReleaseJob(u.Type, u.Spec)
}

type DeleteReleaseJobUpdater struct {
	ID string `json:"id"`
}

func NewDeleteReleaseJobUpdater(args *UpdateReleasePlanArgs) (*DeleteReleaseJobUpdater, error) {
	var updater DeleteReleaseJobUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *DeleteReleaseJobUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	for i, job := range plan.Jobs {
		if job.ID == u.ID {
			plan.Jobs = append(plan.Jobs[:i], plan.Jobs[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("job %s not found", u.ID)
}

func (u *DeleteReleaseJobUpdater) Lint() error {
	if u.ID == "" {
		return fmt.Errorf("id cannot be empty")
	}
	return nil
}

type UpdateApprovalUpdater struct {
	Approval *models.Approval `json:"approval"`
}

func NewUpdateApprovalUpdater(args *UpdateReleasePlanArgs) (*UpdateApprovalUpdater, error) {
	var updater UpdateApprovalUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *UpdateApprovalUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	plan.Approval = u.Approval
	return nil
}

func (u *UpdateApprovalUpdater) Lint() error {
	if u.Approval == nil {
		return fmt.Errorf("approval cannot be empty")
	}
	return lintApproval(u.Approval)
}
