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

	commondb "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
)

func ListLabelsBinding() ([]*models.LabelBinding, error) {
	return mongodb.NewLabelBindingColl().ListByOpt(&mongodb.LabelBindingCollFindOpt{})
}

func CreateLabelsBinding(binding *models.LabelBinding, userName string, logger *zap.SugaredLogger) error {
	//  check label exist

	if _, err := mongodb.NewLabelColl().Find(binding.LabelID); err != nil {
		logger.Errorf("find label err:%s", err)
		return err
	}
	//check resource exist
	switch binding.ResourceType {
	case string(config.ResourceTypeWorkflow):
		_, err := commondb.NewWorkflowColl().FindByID(binding.ResourceID)
		if err != nil {
			logger.Errorf("can not find related resource err:%s", err)
			return err
		}
	case string(config.ResourceTypeProduct):
		_, err := commondb.NewProductColl().FindEnv(&commondb.ProductEnvFindOptions{
			ID: binding.ResourceID,
		})
		if err != nil {
			logger.Errorf("can not find related resource err:%s", err)
			return err
		}
	}
	binding.CreateBy = userName
	return mongodb.NewLabelBindingColl().Create(binding)
}

func DeleteLabelsBinding(id string) error {
	return mongodb.NewLabelBindingColl().Delete(id)
}
