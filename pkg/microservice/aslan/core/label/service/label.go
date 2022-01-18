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
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type ResourceLabel struct {
	ResourceName string `json:"resource_name"`
	ProjectName  string `json:"project_name"`
	ResourceType string `json:"resource_type"`
}

type LabelResource struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	ResourceID string `json:"resource_id"`
}

func CreateLabels(labels []*models.Label) error {
	return mongodb.NewLabelColl().BulkCreate(labels)
}

type ListLabelsArgs struct {
	Labels []mongodb.Label `json:"labels"`
}

type CreateLabelsArgs struct {
	Labels []mongodb.Label `json:"labels"`
}

func ListLabels(args *ListLabelsArgs) ([]*models.Label, error) {
	return mongodb.NewLabelColl().List(mongodb.ListLabelOpt{args.Labels})
}

type ListResourceByLabelsReq struct {
	LabelFilters []mongodb.Label `json:"label_filters"`
}

func ListResourcesByLabels(filters []mongodb.Label, logger *zap.SugaredLogger) (map[string][]mongodb.Resource, error) {
	res := make(map[string][]mongodb.Resource)
	// 1.find labels by label filters
	labels, err := mongodb.NewLabelColl().List(mongodb.ListLabelOpt{Labels: filters})
	if err != nil {
		logger.Errorf("labels ListByOpt err:%s", err)
		return nil, err
	}
	if len(labels) == 0 {
		return res, nil
	}
	// 2.find labelBindings by label ids
	labelIDSet := sets.NewString()
	labelsM := make(map[string]string)
	for _, v := range labels {
		labelIDSet.Insert(v.ID.Hex())
		labelsM[v.ID.Hex()] = fmt.Sprintf("%s-%s", v.Key, v.Value)
	}

	labelBindings, err := mongodb.NewLabelBindingColl().ListByOpt(&mongodb.LabelBindingCollFindOpt{LabelIDs: labelIDSet.List()})
	if err != nil {
		logger.Errorf("labelBindings ListByOpt err:%s", err)
		return nil, err
	}

	// 3.find labels by resourceName-projectName
	for _, v := range labelBindings {
		resource := mongodb.Resource{
			Name:        v.ResourceName,
			ProjectName: v.ProjectName,
			Type:        v.ResourceType,
		}
		labelString, _ := labelsM[v.LabelID]
		if resources, ok := res[labelString]; ok {
			res[labelString] = append(resources, resource)
		} else {
			res[labelString] = []mongodb.Resource{resource}
		}
	}

	return res, nil
}

type ListLabelsByResourcesReq struct {
	Resources []mongodb.Resource `json:"resources"`
}

func ListLabelsByResources(resources []mongodb.Resource, logger *zap.SugaredLogger) (map[string][]*models.Label, error) {
	//1. find the labelBindings by resources
	labelBindings, err := mongodb.NewLabelBindingColl().ListByResources(mongodb.ListLabelBindingsByResources{Resources: resources})
	if err != nil {
		return nil, err
	}
	//2.find labels by labelBindings
	labelIDSet := sets.NewString()
	for _, v := range labelBindings {
		labelIDSet.Insert(v.LabelID)
	}
	labels, err := mongodb.NewLabelColl().ListByIDs(labelIDSet.List())
	if err != nil {
		return nil, err
	}
	labelM := make(map[string]*models.Label)
	for _, label := range labels {
		labelM[label.ID.Hex()] = label
	}
	// 3. iterate resources
	res := make(map[string][]*models.Label)
	for _, labelBinding := range labelBindings {
		resourceKey := fmt.Sprintf("%s-%s", labelBinding.ResourceType, labelBinding.ResourceName)

		label, ok := labelM[labelBinding.LabelID]
		if !ok {
			return nil, fmt.Errorf("can not find label %v", label)
		}

		if arr, ok := res[resourceKey]; ok {
			res[resourceKey] = append(arr, label)
		} else {
			res[resourceKey] = []*models.Label{label}
		}
	}

	return res, nil
}

type DeleteLabelsArgs struct {
	IDs []string
}

func DeleteLabels(ids []string, forceDelete bool, logger *zap.SugaredLogger) error {
	if len(ids) == 0 {
		return nil
	}

	res, err := mongodb.NewLabelBindingColl().ListByOpt(&mongodb.LabelBindingCollFindOpt{LabelIDs: ids})
	if err != nil {
		logger.Errorf("list labelbingding err:%s", err)
		return err
	}

	if !forceDelete && len(res) > 0 {
		return e.ErrForbidden.AddDesc("some label has already bind resource, can not delete")
	}

	if forceDelete {
		var labelBindingIDs []string
		for _, labelBinding := range res {
			labelBindingIDs = append(ids, labelBinding.ID.Hex())
		}

		if err := mongodb.NewLabelBindingColl().BulkDelete(labelBindingIDs); err != nil {
			logger.Errorf("NewLabelBindingColl DeleteMany err :%s", err)
			return err
		}
		return mongodb.NewLabelColl().BulkDelete(ids)
	}

	return mongodb.NewLabelColl().BulkDelete(ids)
}
