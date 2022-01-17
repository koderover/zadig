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
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type LabelFilter struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

type ResourceLabel struct {
	ResourceID   string  `json:"resource_id"`
	ResourceType string  `json:"resource_type"`
	Labels       []Label `json:"labels"`
}

type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
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
	Key    string   `json:"key" form:"key"`
	Values []string `json:"values" form:"values"`
}

func ListLabels(args []*ListLabelsArgs) ([]*models.Label, error) {
	opts := make([]*mongodb.ListLabelOpt, 0)
	for _, v := range args {
		opt := &mongodb.ListLabelOpt{
			Key:    v.Key,
			Values: v.Values,
		}
		opts = append(opts, opt)
	}

	return mongodb.NewLabelColl().Filter(opts)
}

type ListResourceByLabelsReq struct {
	LabelFilters []LabelFilter `json:"label_filters"`
}

func ListResourcesByLabels(filter []LabelFilter, logger *zap.SugaredLogger) ([]ResourceLabel, error) {
	// find the label id
	labelM := map[string]*models.Label{}
	for _, v := range filter {
		labels, err := mongodb.NewLabelColl().List(&mongodb.ListLabelOpt{
			Key:    v.Key,
			Values: v.Values,
		})
		if err != nil {
			continue
		}
		for _, label := range labels {
			labelM[label.ID.Hex()] = label
		}
	}
	labelIds := []string{}
	for k, _ := range labelM {
		labelIds = append(labelIds, k)
	}
	// find the labelBindings
	labelBindings, err := mongodb.NewLabelBindingColl().ListByOpt(&mongodb.LabelBindingCollFindOpt{LabelIDs: labelIds})
	if err != nil {
		logger.Errorf("list labelbinding err:%s", err)
		return nil, err
	}

	var res []ResourceLabel
	for _, v := range labelBindings {
		labelResource, err := ListLabelsByResourceID(v.ResourceID, v.ResourceType, logger)
		if err != nil {
			continue
		}
		var labels []Label
		for _, v := range labelResource {
			label := Label{
				Key:   v.Key,
				Value: v.Value,
			}
			labels = append(labels, label)
		}

		res = append(res, ResourceLabel{
			ResourceID:   v.ResourceID,
			ResourceType: v.ResourceType,
			Labels:       labels,
		})
	}

	return res, nil
}

type ListLabelsByResourceReq struct {
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
}

type ListLabelsByResourcesReq struct {
	ResourceIDs  []string `json:"resource_ids"`
	ResourceType string   `json:"resource_type"`
}

func ListLabelsByResourceID(resourceID, resourceType string, logger *zap.SugaredLogger) ([]LabelResource, error) {
	labelBindings, err := mongodb.NewLabelBindingColl().ListByOpt(&mongodb.LabelBindingCollFindOpt{
		ResourceID:   resourceID,
		ResourceType: resourceType,
	})
	if err != nil {
		logger.Errorf("list labelbinding err:%s", err)
		return nil, err
	}
	var labelResources []LabelResource
	for _, v := range labelBindings {
		label, err := mongodb.NewLabelColl().Find(v.LabelID)
		if err != nil {
			continue
		}
		labelResources = append(labelResources, LabelResource{
			Key:        label.Key,
			Value:      label.Value,
			ResourceID: v.ResourceID,
		})
	}
	return labelResources, nil
}

func ListLabelsByResourceIDs(resources *ListLabelsByResourcesReq, logger *zap.SugaredLogger) (map[string][]LabelResource, error) {
	labelBindings, err := mongodb.NewLabelBindingColl().ListByOpt(&mongodb.LabelBindingCollFindOpt{
		ResourcesIDs: resources.ResourceIDs,
		ResourceType: resources.ResourceType,
	})
	if err != nil {
		logger.Errorf("list labelbinding err:%s", err)
		return nil, err
	}
	labelIDs := sets.NewString()
	for _, v := range labelBindings {
		labelIDs.Insert(v.LabelID)
	}
	labelM := make(map[string]*models.Label, 0)
	labels, err := mongodb.NewLabelColl().List(&mongodb.ListLabelOpt{
		IDs: labelIDs.List(),
	})
	for _, v := range labels {
		labelM[v.ID.Hex()] = v
	}
	if err != nil {
		return nil, err
	}
	resources2LabelResourceM := make(map[string][]LabelResource, 0)
	for _, v := range labelBindings {
		if label, ok := labelM[v.LabelID]; ok {
			resources2LabelResourceM[v.ResourceID] = append(resources2LabelResourceM[v.ResourceID], LabelResource{
				Key:        label.Key,
				Value:      label.Value,
				ResourceID: v.ResourceID,
			})
		}

	}
	return resources2LabelResourceM, nil
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
	if forceDelete {
		var ids []string
		for _, labelBindings := range res {
			ids = append(ids, labelBindings.ID.Hex())
		}

		if err := mongodb.NewLabelBindingColl().BulkDelete(ids); err != nil {
			logger.Errorf("NewLabelBindingColl DeleteMany err :%s", err)
			return err
		}
	}
	if len(res) > 0 && !forceDelete {
		return e.ErrForbidden.AddDesc("some label has already bind resource, can not delete")
	}
	return mongodb.NewLabelColl().BulkDelete(ids)
}

func DeleteLabel(id string, forceDelete bool, logger *zap.SugaredLogger) error {

	// query if the label already bind  resources
	res, err := mongodb.NewLabelBindingColl().ListByOpt(&mongodb.LabelBindingCollFindOpt{LabelID: id})
	if err != nil {
		logger.Errorf("list labelbingding err:%s", err)
		return err
	}
	// force delete : delete related labelBindings
	if forceDelete {
		var ids []string
		for _, labelBindings := range res {
			ids = append(ids, labelBindings.ID.Hex())
		}

		if err := mongodb.NewLabelBindingColl().BulkDelete(ids); err != nil {
			logger.Errorf("NewLabelBindingColl DeleteMany err :%s", err)
			return err
		}
	}
	// non force delete : can not delete label when label has bind resources
	if len(res) > 0 && !forceDelete {
		logger.Error("the label has bind resources,can not delete")
		return e.ErrForbidden.AddDesc("the label has bind resources")
	}
	return mongodb.NewLabelColl().Delete(id)
}
