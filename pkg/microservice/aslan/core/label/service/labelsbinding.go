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

package service

import (
	"fmt"

	"go.uber.org/zap"

	commondb "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListLabelsBinding() ([]*models.LabelBinding, error) {
	return mongodb.NewLabelBindingColl().ListByOpt(&mongodb.LabelBindingCollFindOpt{})
}

type CreateLabelBindingsArgs struct {
	ResourceType string   `bson:"resource_type"               json:"resource_type"`
	ResourceIDs  []string `bson:"resource_ids"                 json:"resource_ids"`
	LabelID      string   `bson:"label_id"                    json:"label_id"`
	CreateBy     string   `bson:"create_by"                   json:"create_by"`
	CreateTime   int64    `bson:"create_time"                 json:"create_time"`
}

func CreateLabelsBinding(cr *CreateLabelBindingsArgs, logger *zap.SugaredLogger) error {
	//  check label exist
	if _, err := mongodb.NewLabelColl().Find(cr.LabelID); err != nil {
		logger.Errorf("find label err:%s", err)
		return err
	}
	//check resource exist
	switch cr.ResourceType {
	case string(config.ResourceTypeWorkflow):
		wks, err := commondb.NewWorkflowColl().List(&commondb.ListWorkflowOption{
			Ids: cr.ResourceIDs,
		})
		if err != nil {
			logger.Errorf("can not find related resource err:%s", err)
			return err
		}
		if len(wks) != len(cr.ResourceIDs) {
			logger.Errorf("can not find related resource err:%s", err)
			return e.ErrForbidden.AddDesc("can not find related resource")
		}
	case string(config.ResourceTypeProduct):
		prs, err := commondb.NewProductColl().List(&commondb.ProductListOptions{
			InIDs: cr.ResourceIDs,
		})
		if err != nil {
			logger.Errorf("can not find related resource err:%s", err)
			return err
		}
		if len(prs) != len(cr.ResourceIDs) {
			logger.Errorf("can not find related resource err:%s", err)
			return e.ErrForbidden.AddDesc("can not find related resource")
		}
	default:
		errMsg := fmt.Sprintf("resource type not allowed :%s", cr.ResourceType)
		return e.ErrInternalError.AddDesc(errMsg)
	}
	var labelBindings []*models.LabelBinding
	for _, v := range cr.ResourceIDs {
		labelBinding := &models.LabelBinding{
			ResourceType: cr.ResourceType,
			ResourceID:   v,
			LabelID:      cr.LabelID,
			CreateBy:     cr.CreateBy,
		}
		labelBindings = append(labelBindings, labelBinding)
	}
	return mongodb.NewLabelBindingColl().CreateMany(labelBindings)
}

type DeleteLabelsBindingsArgs struct {
	IDs []string
}

func DeleteLabelsBindings(ids []string) error {
	return mongodb.NewLabelBindingColl().BulkDelete(ids)
}
