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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
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
	Description         string           `bson:"description"       yaml:"description"                   json:"description"`
	Approval            *models.Approval `bson:"approval"       yaml:"approval"                   json:"approval,omitempty"`
}

func OpenAPICreateReleasePlan(c *handler.Context, rawArgs *OpenAPICreateReleasePlanArgs) error {
	args := &models.ReleasePlan{
		Name:        rawArgs.Name,
		Manager:     rawArgs.Manager,
		StartTime:   rawArgs.StartTime,
		EndTime:     rawArgs.EndTime,
		Description: rawArgs.Description,
		Approval:    rawArgs.Approval,
	}
	if args.Name == "" || args.Manager == "" {
		return errors.New("Required parameters are missing")
	}
	if err := lintReleaseTimeRange(args.StartTime, args.EndTime); err != nil {
		return errors.Wrap(err, "lint release time range error")
	}
	searchUserResp, err := user.New().SearchUser(&user.SearchUserArgs{
		Account:      args.Manager,
		IdentityType: rawArgs.ManagerIdentityType,
	})
	if err != nil {
		return errors.Errorf("Failed to get user %s, error: %v", args.Manager, err)
	}
	if len(searchUserResp.Users) == 0 {
		return errors.Errorf("User %s not found", args.Manager)
	}
	if len(searchUserResp.Users) > 1 {
		return errors.Errorf("User %s search failed", args.Manager)
	}
	args.ManagerID = searchUserResp.Users[0].UID

	if args.Approval != nil {
		if err := lintApproval(args.Approval); err != nil {
			return errors.Errorf("lintApproval error: %v", err)
		}
		if args.Approval.Type == config.LarkApproval {
			if err := createLarkApprovalDefinition(args.Approval.LarkApproval); err != nil {
				return errors.Errorf("createLarkApprovalDefinition error: %v", err)
			}
		}
	}

	nextID, err := mongodb.NewCounterColl().GetNextSeq(setting.ReleasePlanFmt)
	if err != nil {
		log.Errorf("OpenAPICreateReleasePlan.GetNextSeq error: %v", err)
		return e.ErrGetCounter.AddDesc(err.Error())
	}
	args.Index = nextID
	args.CreatedBy = c.UserName
	args.UpdatedBy = c.UserName
	args.CreateTime = time.Now().Unix()
	args.UpdateTime = time.Now().Unix()
	args.Status = config.StatusPlanning

	planID, err := mongodb.NewReleasePlanColl().Create(args)
	if err != nil {
		return errors.Wrap(err, "create release plan error")
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

	return nil
}
