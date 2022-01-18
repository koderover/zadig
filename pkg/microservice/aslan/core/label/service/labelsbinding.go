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

type CreateLabelBinding struct {
	Resource   mongodb.Resource `json:"resource"`
	LabelID    string           `bson:"label_id"                    json:"label_id"`
	CreateBy   string           `bson:"create_by"                   json:"create_by"`
	CreateTime int64            `bson:"create_time"                 json:"create_time"`
}

type CreateLabelBindingsArgs struct {
	CreateLabelBindings []CreateLabelBinding `json:"create_label_bindings"`
}

func CreateLabelBindings(cr *CreateLabelBindingsArgs, userName string, logger *zap.SugaredLogger) error {
	//  check label exist
	labelIDs := []string{}
	for _, binding := range cr.CreateLabelBindings {
		labelIDs = append(labelIDs, binding.LabelID)
		binding.CreateBy = userName
	}

	labels, err := mongodb.NewLabelColl().ListByIDs(labelIDs)
	if err != nil {
		logger.Errorf("find labels err:%s", err)
		return err
	}
	if len(labels) != len(labelIDs) {
		logger.Errorf("there're labels not exist")
		return fmt.Errorf("there're labels not exist")
	}
	//check resource exist
	m := make(map[string][]CreateLabelBinding)
	for _, binding := range cr.CreateLabelBindings {
		m[binding.Resource.Type] = append(m[binding.Resource.Type], binding)
	}
	for k, v := range m {
		switch k {
		case string(config.ResourceTypeWorkflow):
			workflows := []commondb.Workflow{}
			for _, vv := range v {
				workflow := commondb.Workflow{
					Name:        vv.Resource.Name,
					ProjectName: vv.Resource.ProjectName,
				}
				workflows = append(workflows, workflow)
			}
			wks, err := commondb.NewWorkflowColl().ListByWorkflows(commondb.ListWorkflowOpt{workflows})
			if err != nil {
				logger.Errorf("can not find related resource err:%s", err)
				return err
			}
			if len(wks) != len(v) {
				logger.Errorf("there're resources not exist")
				return e.ErrForbidden.AddDesc("there're resources not exist")
			}
		case string(config.ResourceTypeProduct):
			products := []commondb.Product{}
			for _, vv := range v {
				product := commondb.Product{
					Name:        vv.Resource.Name,
					ProjectName: vv.Resource.ProjectName,
				}
				products = append(products, product)
			}
			pros, err := commondb.NewProductColl().ListByProducts(commondb.ListProductOpt{products})
			if err != nil {
				logger.Errorf("can not find related resource err:%s", err)
				return err
			}
			if len(pros) != len(v) {
				logger.Errorf("there're resources not exist")
				return e.ErrForbidden.AddDesc("there're resources not exist")
			}
		}
	}

	var labelBindings []*models.LabelBinding
	for _, v := range cr.CreateLabelBindings {
		labelBinding := &models.LabelBinding{
			ResourceType: v.Resource.Type,
			ResourceName: v.Resource.Name,
			ProjectName:  v.Resource.ProjectName,
			LabelID:      v.LabelID,
			CreateBy:     v.CreateBy,
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
