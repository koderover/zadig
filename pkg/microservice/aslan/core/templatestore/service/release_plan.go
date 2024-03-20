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

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func CreateReleasePlanTemplate(userName string, template *commonmodels.ReleasePlanTemplate, logger *zap.SugaredLogger) error {
	if _, err := commonrepo.NewReleasePlanTemplateColl().Find(&commonrepo.ReleasePlanTemplateQueryOption{Name: template.TemplateName}); err == nil {
		errMsg := fmt.Sprintf("发布计划模板名称: %s 已存在", template.TemplateName)
		logger.Error(errMsg)
		return e.ErrCreateReleasePlanTemplate.AddDesc(errMsg)
	}
	template.CreatedBy = userName
	template.UpdatedBy = userName

	if err := commonrepo.NewReleasePlanTemplateColl().Create(template); err != nil {
		errMsg := fmt.Sprintf("Failed to create release plan template %s, err: %v", template.TemplateName, err)
		logger.Error(errMsg)
		return e.ErrCreateReleasePlanTemplate.AddDesc(errMsg)
	}
	return nil
}

func UpdateReleasePlanTemplate(userName string, template *commonmodels.ReleasePlanTemplate, logger *zap.SugaredLogger) error {
	if _, err := commonrepo.NewReleasePlanTemplateColl().Find(&commonrepo.ReleasePlanTemplateQueryOption{ID: template.ID.Hex()}); err != nil {
		errMsg := fmt.Sprintf("release plan template %s not found: %v", template.TemplateName, err)
		logger.Error(errMsg)
		return e.ErrUpdateReleasePlanTemplate.AddDesc(errMsg)
	}
	template.UpdatedBy = userName

	if err := commonrepo.NewReleasePlanTemplateColl().Update(template); err != nil {
		errMsg := fmt.Sprintf("Failed to update release plan template %s, err: %v", template.TemplateName, err)
		logger.Error(errMsg)
		return e.ErrUpdateReleasePlanTemplate.AddDesc(errMsg)
	}
	return nil
}

func ListReleasePlanTemplate(logger *zap.SugaredLogger) ([]*commonmodels.ReleasePlanTemplate, error) {
	templates, err := commonrepo.NewReleasePlanTemplateColl().List(&commonrepo.ReleasePlanTemplateListOption{})
	if err != nil {
		errMsg := fmt.Sprintf("Failed to list release plan template err: %v", err)
		logger.Error(errMsg)
		return nil, e.ErrListReleasePlanTemplate.AddDesc(errMsg)
	}
	return templates, nil
}

func GetReleasePlanTemplateByID(idStr string, logger *zap.SugaredLogger) (*commonmodels.ReleasePlanTemplate, error) {
	template, err := commonrepo.NewReleasePlanTemplateColl().Find(&commonrepo.ReleasePlanTemplateQueryOption{ID: idStr})
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get release plan template err: %v", err)
		logger.Error(errMsg)
		return template, e.ErrGetReleasePlanTemplate.AddDesc(errMsg)
	}
	return template, nil
}

func DeleteReleasePlanTemplateByID(idStr string, logger *zap.SugaredLogger) error {
	if err := commonrepo.NewReleasePlanTemplateColl().DeleteByID(idStr); err != nil {
		errMsg := fmt.Sprintf("Failed to delete release plan template err: %v", err)
		logger.Error(errMsg)
		return e.ErrDeleteReleasePlanTemplate.AddDesc(errMsg)
	}
	return nil
}
