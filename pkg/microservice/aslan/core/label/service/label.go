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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/dto"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CreateLabels(labels []*models.Label) error {
	return mongodb.NewLabelColl().BulkCreate(labels)
}

func ListLabels(key string, values []string, labelType string) ([]*models.Label, error) {
	opt := &mongodb.ListLabelOpt{
		Key:    key,
		Values: values,
		Type:   labelType,
	}
	return mongodb.NewLabelColl().List(opt)
}

func ListResourcesByLabels(filter []dto.LabelFilter, logger *zap.SugaredLogger) ([]dto.ResourceLabel, error) {
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

	var res []dto.ResourceLabel
	for _, v := range labelBindings {
		labelResource, err := ListLabelsByResourceID(v.ResourceID, v.ResourceType, logger)
		if err != nil {
			continue
		}
		var labels []dto.Label
		for _, v := range labelResource {
			label := dto.Label{
				Key:   v.Key,
				Value: v.Value,
			}
			labels = append(labels, label)
		}

		res = append(res, dto.ResourceLabel{
			ResourceID:   v.ResourceID,
			ResourceType: v.ResourceType,
			Labels:       labels,
		})
	}

	return res, nil
}

func ListLabelsByResourceID(resourceID, resourceType string, logger *zap.SugaredLogger) ([]dto.LabelResource, error) {
	labelBindings, err := mongodb.NewLabelBindingColl().ListByOpt(&mongodb.LabelBindingCollFindOpt{
		ResourceID:   resourceID,
		ResourceType: resourceType,
	})
	if err != nil {
		logger.Errorf("list labelbinding err:%s", err)
		return nil, err
	}
	var labelResources []dto.LabelResource
	for _, v := range labelBindings {
		label, err := mongodb.NewLabelColl().Find(v.LabelID)
		if err != nil {
			continue
		}
		labelResources = append(labelResources, dto.LabelResource{
			Key:        label.Key,
			Value:      label.Value,
			ResourceID: v.ResourceID,
		})
	}
	return labelResources, nil
}

func DeleteLabel(id string, force bool, logger *zap.SugaredLogger) error {

	// query if the label already bind  resources
	res, err := mongodb.NewLabelBindingColl().ListByOpt(&mongodb.LabelBindingCollFindOpt{LabelID: id})
	if err != nil {
		logger.Errorf("list labelbingding err:%s", err)
		return err
	}
	// force delete : delete related labelBindings
	if force {
		var ids []string
		for _, labelBindings := range res {
			ids = append(ids, labelBindings.ID.Hex())
		}

		if err := mongodb.NewLabelBindingColl().DeleteMany(ids); err != nil {
			logger.Errorf("NewLabelBindingColl DeleteMany err :%s", err)
			return err
		}
	}
	// non force delete : can not delete label when label has bind resources
	if len(res) > 0 && !force {
		logger.Error("the label has bind resources,can not delete")
		return e.ErrForbidden.AddDesc("the label has bind resources")
	}
	return mongodb.NewLabelColl().Delete(id)
}
