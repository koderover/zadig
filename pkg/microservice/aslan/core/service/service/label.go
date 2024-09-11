/*
Copyright 2024 The KodeRover Authors.

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
	"context"
	"fmt"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/setting"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"github.com/koderover/zadig/v2/pkg/util/boolptr"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func AddServiceLabel(labelID, projectKey, serviceName string, production *bool, value string, log *zap.SugaredLogger) error {
	if len(labelID) == 0 ||
		len(projectKey) == 0 ||
		len(serviceName) == 0 ||
		production == nil {
		log.Errorf("invalid create service label parameter, labelID: %s, projectKey: %s, serviceName: %s, production: %+v", labelID, projectKey, serviceName, production)
		return fmt.Errorf("invalid create service label parameter")
	}

	// validate if the service exist
	opt := &commonrepo.ServiceFindOption{
		ServiceName:   serviceName,
		ProductName:   projectKey,
		ExcludeStatus: setting.ProductStatusDeleting,
	}

	session := mongotool.Session()
	defer session.EndSession(context.TODO())

	serviceColl := repository.ServiceCollWithSession(*production, session)

	_, err := serviceColl.Find(opt)

	if err != nil {
		log.Errorf("failed to validate if the service exist, error: %s", err)
		return fmt.Errorf("failed to validate if the service exist, error: %s", err)
	}

	arg := &commonmodels.LabelBinding{
		LabelID:     labelID,
		ServiceName: serviceName,
		Production:  boolptr.IsTrue(production),
		ProjectKey:  projectKey,
		Value:       value,
	}

	err = commonrepo.NewLabelBindingColl().Create(arg)
	if err != nil {
		log.Errorf("failed to create service label binding, error: %s", err)
		return err
	}

	return nil
}

func UpdateServiceLabel(bindingID, value string, log *zap.SugaredLogger) error {
	err := commonrepo.NewLabelBindingColl().Update(bindingID, &commonmodels.LabelBinding{
		Value: value,
	})

	if err != nil {
		log.Errorf("failed to update service label binding, error: %s", err)
		return err
	}

	return nil
}

func DeleteServiceLabel(bindingID string, log *zap.SugaredLogger) error {
	err := commonrepo.NewLabelBindingColl().DeleteByID(bindingID)

	if err != nil {
		log.Errorf("failed to delete service label, error: %s", err)
		return err
	}

	return nil
}

type ServiceLabelResp struct {
	ID    string `json:"id,omitempty"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

func ListServiceLabels(projectKey, serviceName string, production *bool, log *zap.SugaredLogger) ([]*ServiceLabelResp, error) {
	resp := make([]*ServiceLabelResp, 0)

	labelbindings, err := commonrepo.NewLabelBindingColl().List(&commonrepo.LabelBindingListOption{
		ServiceName: serviceName,
		ProjectKey:  projectKey,
		Production:  production,
	})

	if err != nil {
		log.Errorf("failed to list label bindings for project: %s, service: %s, error: %s", projectKey, serviceName, err)
		return nil, fmt.Errorf("failed to list label bindings for project: %s, service: %s, error: %s", projectKey, serviceName, err)
	}

	for _, binding := range labelbindings {
		// TODO: query the data all at once
		labelSetting, err := commonrepo.NewLabelColl().GetByID(binding.LabelID)
		if err != nil {
			log.Errorf("failed to find the label setting for id: %s", binding.LabelID)
			return nil, fmt.Errorf("failed to find the label setting for id: %s", binding.LabelID)
		}

		resp = append(resp, &ServiceLabelResp{
			ID:    binding.ID.Hex(),
			Key:   labelSetting.Key,
			Value: binding.Value,
		})
	}

	return resp, nil
}

func GetLabelBindingInfo(bindingID string) (*commonmodels.LabelBinding, error) {
	return commonrepo.NewLabelBindingColl().GetByID(bindingID)
}
