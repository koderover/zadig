/*
Copyright 2023 The KodeRover Authors.

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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/template"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type ScanningTemplateBrief struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type ScanningTemplateListResp struct {
	ScanningTemplates []*ScanningTemplateBrief `json:"scanning_templates"`
	Total             int                      `json:"total,omitempty"`
}

func CreateScanningTemplate(userName string, template *commonmodels.ScanningTemplate, logger *zap.SugaredLogger) error {
	if len(template.Name) == 0 {
		return e.ErrCreateScanningModule.AddDesc("empty name")
	}

	// prevent panic
	if template.AdvancedSetting != nil {
		if err := commonutil.CheckDefineResourceParam(template.AdvancedSetting.ResReq, template.AdvancedSetting.ResReqSpec); err != nil {
			return e.ErrCreateScanningModule.AddDesc(err.Error())
		}
	}

	template.UpdatedBy = userName
	if err := commonrepo.NewScanningTemplateColl().Create(template); err != nil {
		logger.Errorf("create scanning template %s error: %s", template.Name, err)
		return e.ErrCreateScanningModule.AddErr(err)
	}
	return nil
}

func UpdateScanningTemplate(id string, template *commonmodels.ScanningTemplate, logger *zap.SugaredLogger) error {
	_, err := commonrepo.NewScanningTemplateColl().Find(&commonrepo.ScanningTemplateQueryOption{
		ID: id,
	})

	if err != nil {
		logger.Errorf("failed to find the scanning template to be updated, id: %s, error: %s", id, err)
		return e.ErrCreateScanningModule.AddDesc(fmt.Sprintf("failed to find the given scanning template of id: %s, error: %s", template.ID, err))
	}

	if template.AdvancedSetting != nil {
		if err := commonutil.CheckDefineResourceParam(template.AdvancedSetting.ResReq, template.AdvancedSetting.ResReqSpec); err != nil {
			return e.ErrCreateScanningModule.AddDesc(err.Error())
		}
	}

	return commonrepo.NewScanningTemplateColl().Update(id, template)
}

func GetScanningTemplateByID(idStr string) (*commonmodels.ScanningTemplate, error) {
	return commonrepo.NewScanningTemplateColl().Find(&commonrepo.ScanningTemplateQueryOption{ID: idStr})
}

func ListScanningTemplates(pageNum, pageSize int) (*ScanningTemplateListResp, error) {
	buildTemplates, count, err := commonrepo.NewScanningTemplateColl().List(pageNum, pageSize)
	if err != nil {
		return nil, err
	}
	ret := &ScanningTemplateListResp{
		Total: count,
	}

	for _, bt := range buildTemplates {
		ret.ScanningTemplates = append(ret.ScanningTemplates, &ScanningTemplateBrief{
			Id:   bt.ID.Hex(),
			Name: bt.Name,
		})
	}

	return ret, nil
}

func DeleteScanningTemplate(id string, logger *zap.SugaredLogger) error {
	_, err := GetScanningTemplateByID(id)
	if err != nil {
		logger.Errorf("failed to find scanning template with id: %s, err: %s", id, err)
		return fmt.Errorf("failed to find scanning template with id: %s, err: %s", id, err)
	}

	// when template build is used by particular builds, this template can't be deleted
	_, count, err := commonrepo.NewScanningColl().List(&commonrepo.ScanningListOption{
		TemplateID: id,
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to find builds with template id: %s, err: %s", id, err)
	}
	if count > 0 {
		logger.Errorf("scanning template of id: %s is in use, it cannot be deleted", id)
		return fmt.Errorf("template build has beed used, can't be deleted")
	}

	err = commonrepo.NewScanningTemplateColl().DeleteByID(id)
	if err != nil {
		logger.Errorf("Failed to delete build template %s, err: %s", id, err)
		return err
	}
	return nil
}

func GetScanningTemplateReference(id string, logger *zap.SugaredLogger) ([]*template.ScanningTemplateReference, error) {
	ret := make([]*template.ScanningTemplateReference, 0)
	referenceList, err := commonrepo.NewScanningColl().GetScanningTemplateReference(id)
	if err != nil {
		logger.Errorf("Failed to get build template reference for template id: %s, the error is: %s", id, err)
		return ret, err
	}

	for _, reference := range referenceList {
		ret = append(ret, &template.ScanningTemplateReference{
			ScanningName: reference.Name,
			ProjectName:  reference.ProjectName,
		})
	}
	return ret, nil
}
