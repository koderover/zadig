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
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	commondb "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type CreateLabelBindingsArgs struct {
	LabelBindings []*mongodb.LabelBinding `json:"label_bindings"`
}

func CreateLabelBindings(cr *CreateLabelBindingsArgs, userName string, logger *zap.SugaredLogger) error {
	//  check label exist
	labelIDs := sets.String{}
	for _, binding := range cr.LabelBindings {
		labelIDs.Insert(binding.LabelID)
		binding.CreateBy = userName
	}

	labels, err := mongodb.NewLabelColl().ListByIDs(labelIDs.List())
	if err != nil {
		logger.Errorf("find labels err:%s", err)
		return err
	}
	if len(labels) != len(labelIDs) {
		logger.Errorf("there're labels not exist")
		return fmt.Errorf("there're labels not exist")
	}
	//check resource exist
	m := make(map[string][]*mongodb.LabelBinding)
	for _, binding := range cr.LabelBindings {
		m[binding.Resource.Type] = append(m[binding.Resource.Type], binding)
	}
	for k, v := range m {
		switch k {
		case string(config.ResourceTypeWorkflow):
			workflows := []commondb.Workflow{}
			resourceSet := sets.NewString()
			for _, vv := range v {
				workflow := commondb.Workflow{
					Name:        vv.Resource.Name,
					ProjectName: vv.Resource.ProjectName,
				}
				workflows = append(workflows, workflow)
				resourceSet.Insert(fmt.Sprintf("%s-%s", vv.Resource.Name, vv.Resource.ProjectName))
			}
			wks, err := commondb.NewWorkflowColl().ListByWorkflows(commondb.ListWorkflowOpt{workflows})
			if err != nil {
				logger.Errorf("can not find related resource err:%s", err)
				return err
			}
			if len(wks) != resourceSet.Len() {
				logger.Errorf("there're resources not exist")
				return e.ErrForbidden.AddDesc("there're resources not exist")
			}
		case string(config.ResourceTypeCommonWorkflow):
			workflows := []commondb.WorkflowV4{}
			resourceSet := sets.NewString()
			for _, vv := range v {
				workflow := commondb.WorkflowV4{
					Name:        vv.Resource.Name,
					ProjectName: vv.Resource.ProjectName,
				}
				workflows = append(workflows, workflow)
				resourceSet.Insert(fmt.Sprintf("%s-%s", vv.Resource.Name, vv.Resource.ProjectName))
			}
			wks, err := commondb.NewWorkflowV4Coll().ListByWorkflows(commondb.ListWorkflowV4Opt{workflows})
			if err != nil {
				logger.Errorf("can not find related resource err:%s", err)
				return err
			}
			if len(wks) != resourceSet.Len() {
				wksj, _ := json.Marshal(wks)
				logger.Errorf("there're resources not exist:%s, %s", wksj, resourceSet)
				return e.ErrForbidden.AddDesc("there're resources not exist")
			}
		case string(config.ResourceTypeEnvironment):
			products := []commondb.Product{}
			resourceSet := sets.NewString()
			for _, vv := range v {
				product := commondb.Product{
					Name:        vv.Resource.Name,
					ProjectName: vv.Resource.ProjectName,
				}
				products = append(products, product)
				resourceSet.Insert(fmt.Sprintf("%s-%s", vv.Resource.Name, vv.Resource.ProjectName))
			}
			pros, err := commondb.NewProductColl().ListByProducts(commondb.ListProductOpt{products})
			if err != nil {
				logger.Errorf("can not find related resource err:%s", err)
				return err
			}
			if len(pros) != resourceSet.Len() {
				logger.Errorf("there're resources not exist")
				return e.ErrForbidden.AddDesc("there're resources not exist")
			}
		}
	}

	return mongodb.NewLabelBindingColl().CreateMany(cr.LabelBindings)
}

type DeleteLabelBindingsArgs struct {
	LabelBindings []*mongodb.LabelBinding `json:"label_bindings"`
}

func DeleteLabelBindings(dr *DeleteLabelBindingsArgs, userName string, logger *zap.SugaredLogger) error {
	logger.Infof("DeleteLabelBindings:%v userName:%s", dr, userName)
	return mongodb.NewLabelBindingColl().BulkDelete(dr.LabelBindings)
}

type DeleteLabelsBindingsByIdsArgs struct {
	IDs []string
}

func DeleteLabelsBindingsByIds(ids []string) error {
	return mongodb.NewLabelBindingColl().BulkDeleteByIds(ids)
}
