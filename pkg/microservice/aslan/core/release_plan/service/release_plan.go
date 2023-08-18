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
	"time"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func CreateReleasePlan(creator string, args *models.ReleasePlan) error {
	if args.Name == "" || args.PrincipalID == "" {
		return errors.New("Required parameters are missing")
	}
	if args.StartTime > args.EndTime || args.EndTime < time.Now().Unix() {
		return errors.New("Invalid release time range")
	}
	user, err := orm.GetUserByUid(args.PrincipalID, repository.DB)
	if err != nil {
		return errors.Errorf("Failed to get user by id %s, error: %v", args.PrincipalID, err)
	}
	args.Principal = user.Name

	nextID, err := mongodb.NewCounterColl().GetNextSeq(setting.WorkflowTaskV4Fmt)
	if err != nil {
		log.Errorf("CreateReleasePlan.GetNextSeq error: %v", err)
		return e.ErrGetCounter.AddDesc(err.Error())
	}
	args.Index = nextID
	args.CreatedBy = creator
	args.UpdatedBy = creator
	args.CreateTime = time.Now().Unix()
	args.UpdateTime = time.Now().Unix()

	return mongodb.NewReleasePlanColl().Create(args)
}

type ListReleasePlanResp struct {
	List  []*models.ReleasePlan `json:"list"`
	Total int64
}

func ListReleasePlans(pageNum, pageSize int64) (*ListReleasePlanResp, error) {
	list, total, err := mongodb.NewReleasePlanColl().ListByOptions(&mongodb.ListReleasePlanOption{
		PageNum:        pageNum,
		PageSize:       pageSize,
		IsSort:         true,
		ExcludedFields: []string{"jobs", "approval", "logs"},
	})
	if err != nil {
		return nil, errors.Wrap(err, "ListReleasePlans")
	}
	return &ListReleasePlanResp{
		List:  list,
		Total: total,
	}, nil
}

func GetReleasePlan(id string) (*models.ReleasePlan, error) {
	return mongodb.NewReleasePlanColl().GetByID(id)
}

func DeleteReleasePlan(id string) error {
	return mongodb.NewReleasePlanColl().DeleteByID(id)
}

type UpdateReleasePlanArgs struct {
	Verb string      `json:"verb"`
	Spec interface{} `json:"spec"`
}

func UpdateReleasePlan(planID, username string, args *UpdateReleasePlanArgs) error {
	//todo lock

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		return errors.Wrap(err, "get plan")
	}

	updater, err := NewPlanUpdater(args)
	if err != nil {
		return errors.Wrap(err, "new plan updater")
	}
	if updater.Lint() != nil {
		return errors.Wrap(err, "lint")
	}

	updater.Update(plan)

	plan.UpdatedBy = username
	plan.UpdateTime = time.Now().Unix()

	if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
		return errors.Wrap(err, "update plan")
	}
	return nil
}
