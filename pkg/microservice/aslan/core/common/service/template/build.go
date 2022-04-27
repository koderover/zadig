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

package template

import (
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

type BuildTemplateBrief struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type BuildTemplateListResp struct {
	BuildTemplates []*BuildTemplateBrief `json:"build_templates"`
	Total          int                   `json:"total,omitempty"`
}

func AddBuildTemplate(userName string, build *commonmodels.BuildTemplate, logger *zap.SugaredLogger) error {
	if len(build.Name) == 0 {
		return e.ErrCreateBuildModule.AddDesc("empty name")
	}
	if err := commonutil.CheckDefineResourceParam(build.PreBuild.ResReq, build.PreBuild.ResReqSpec); err != nil {
		return e.ErrCreateBuildModule.AddDesc(err.Error())
	}
	build.UpdateBy = userName
	if err := commonrepo.NewBuildTemplateColl().Create(build); err != nil {
		log.Errorf("[Build.Upsert] %s error: %v", build.Name, err)
		return e.ErrCreateBuildModule.AddErr(err)
	}
	return nil
}

func GetBuildTemplateByName(name string) (*commonmodels.BuildTemplate, error) {
	return mongodb.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{Name: name})
}

func GetBuildTemplateByID(idStr string) (*commonmodels.BuildTemplate, error) {
	return mongodb.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{ID: idStr})
}

func ListBuildTemplates(pageNum, pageSize int) (*BuildTemplateListResp, error) {
	buildTemplates, count, err := mongodb.NewBuildTemplateColl().List(pageNum, pageSize)
	if err != nil {
		return nil, err
	}
	ret := &BuildTemplateListResp{
		Total: count,
	}
	for _, bt := range buildTemplates {
		ret.BuildTemplates = append(ret.BuildTemplates, &BuildTemplateBrief{
			Id:   bt.ID.Hex(),
			Name: bt.Name,
		})
	}
	return ret, nil
}

func RemoveBuildTemplate(name string, logger *zap.SugaredLogger) error {
	err := mongodb.NewBuildTemplateColl().DeleteByID(name)
	if err != nil {
		logger.Errorf("Failed to delete build template %s, err: %s", name, err)
		return err
	}
	return nil
}

func UpdateBuildTemplate(id string, buildTemplate *commonmodels.BuildTemplate, logger *zap.SugaredLogger) error {
	_, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{
		ID: id,
	})
	if err != nil {
		return err
	}
	return commonrepo.NewBuildTemplateColl().Update(id, buildTemplate)
}
