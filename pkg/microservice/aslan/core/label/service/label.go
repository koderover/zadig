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
	Labels []mongodb.Label `json:"labels"`
}

type CreateLabelsArgs struct {
	Labels []mongodb.Label `json:"labels"`
}

func ListLabels(args *ListLabelsArgs) ([]*models.Label, error) {
	return mongodb.NewLabelColl().List(mongodb.ListLabelOpt{args.Labels})
}

type ListResourceByLabelsReq struct {
	LabelFilters []LabelFilter `json:"label_filters"`
}

func ListResourcesByLabels(filter []LabelFilter, logger *zap.SugaredLogger) ([]ResourceLabel, error) {
	// TODO - mouuii
	return nil, nil
}

type ListLabelsByResourceReq struct {
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
}

type ListLabelsByResourcesReq struct {
	ResourceIDs  []string `json:"resource_ids"`
	ResourceType string   `json:"resource_type"`
}

func ListLabelsByResourceIDs(resources *ListLabelsByResourcesReq, logger *zap.SugaredLogger) (map[string][]LabelResource, error) {
	return nil, nil
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
		return mongodb.NewLabelColl().BulkDelete(ids)
	}
	if len(res) > 0 && !forceDelete {
		return e.ErrForbidden.AddDesc("some label has already bind resource, can not delete")
	}
	return nil
}
