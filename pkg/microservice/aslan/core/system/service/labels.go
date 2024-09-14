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
	"fmt"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"go.uber.org/zap"
)

func CreateServiceLabelSetting(args *commonmodels.Label, log *zap.SugaredLogger) error {
	if args == nil {
		return fmt.Errorf("nil label")
	}

	if err := commonrepo.NewLabelColl().Create(args); err != nil {
		log.Errorf("Create service label err:%v", err)
		return err
	}

	return nil
}

func ListServiceLabelSettings(log *zap.SugaredLogger) ([]*commonmodels.Label, error) {
	labels, err := commonrepo.NewLabelColl().List(&commonrepo.LabelListOption{})
	if err != nil {
		log.Errorf("List service label err:%v", err)
		return nil, err
	}

	return labels, nil
}

func UpdateServiceLabelSetting(id string, args *commonmodels.Label, log *zap.SugaredLogger) error {
	err := commonrepo.NewLabelColl().Update(id, args)
	if err != nil {
		log.Errorf("update service label err:%v", err)
		return err
	}

	return nil
}

func DeleteServiceLabelSetting(id string, log *zap.SugaredLogger) error {
	// first make sure the label is not used
	bindings, err := commonrepo.NewLabelBindingColl().List(&commonrepo.LabelBindingListOption{
		ServiceName: "",
		ProjectKey:  "",
		Production:  nil,
		LabelFilter: nil,
	})

	if err != nil {
		log.Errorf("failed to validate if the label is used, error: %s", err)
		return fmt.Errorf("failed to validate if the label is used, error: %s", err)
	}

	if len(bindings) > 0 {
		log.Errorf("label is still being used, cannot delete")
		return fmt.Errorf("label is still being used, cannot delete")
	}

	err = commonrepo.NewLabelColl().Delete(id)
	if err != nil {
		log.Errorf("delete service label err:%v", err)
		return err
	}

	return nil
}
