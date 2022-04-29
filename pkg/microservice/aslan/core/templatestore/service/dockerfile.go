/*
Copyright 2021 The KodeRover Authors.

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
	"errors"
	"strings"

	dockerfileinstructions "github.com/moby/buildkit/frontend/dockerfile/instructions"
	dockerfileparser "github.com/moby/buildkit/frontend/dockerfile/parser"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	. "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
)

func CreateDockerfileTemplate(template *DockerfileTemplate, logger *zap.SugaredLogger) error {
	err := commonrepo.NewDockerfileTemplateColl().Create(&commonmodels.DockerfileTemplate{
		Name:    template.Name,
		Content: template.Content,
	})
	if err != nil {
		logger.Errorf("create dockerfile template error: %s", err)
	}
	return err
}

func UpdateDockerfileTemplate(id string, template *DockerfileTemplate, logger *zap.SugaredLogger) error {
	err := commonrepo.NewDockerfileTemplateColl().Update(
		id,
		&commonmodels.DockerfileTemplate{
			Name:    template.Name,
			Content: template.Content,
		},
	)
	if err != nil {
		logger.Errorf("update dockerfile template error: %s", err)
	}
	return err
}

func ListDockerfileTemplate(pageNum, pageSize int, logger *zap.SugaredLogger) ([]*DockerfileListObject, int, error) {
	resp := make([]*DockerfileListObject, 0)
	templateList, total, err := commonrepo.NewDockerfileTemplateColl().List(pageNum, pageSize)
	if err != nil {
		logger.Errorf("list dockerfile template error: %s", err)
		return resp, 0, err
	}
	for _, obj := range templateList {
		resp = append(resp, &DockerfileListObject{
			ID:   obj.ID.Hex(),
			Name: obj.Name,
		})
	}
	return resp, total, err
}

func DeleteDockerfileTemplate(id string, logger *zap.SugaredLogger) error {
	ref, err := commonrepo.NewBuildColl().GetDockerfileTemplateReference(id)
	if err != nil {
		logger.Errorf("Failed to get build reference for template id: %s, the error is: %s", id, err)
		return err
	}
	if len(ref) > 0 {
		return errors.New("this template is in use")
	}
	err = commonrepo.NewDockerfileTemplateColl().DeleteByID(id)
	if err != nil {
		logger.Errorf("Failed to delete dockerfile template of id: %s, the error is: %s", id, err)
	}
	return err
}

func GetDockerfileTemplateReference(id string, logger *zap.SugaredLogger) ([]*BuildReference, error) {
	ret := make([]*BuildReference, 0)
	referenceList, err := commonrepo.NewBuildColl().GetDockerfileTemplateReference(id)
	if err != nil {
		logger.Errorf("Failed to get build reference for dockerfile template id: %s, the error is: %s", id, err)
		return ret, err
	}
	for _, reference := range referenceList {
		ret = append(ret, &BuildReference{
			BuildName:   reference.Name,
			ProjectName: reference.ProductName,
		})
	}
	return ret, nil
}

func ValidateDockerfileTemplate(template string, logger *zap.SugaredLogger) error {
	// some dockerfile validation stuff
	reader := strings.NewReader(template)
	result, err := dockerfileparser.Parse(reader)
	if err != nil {
		return err
	}
	_, _, err = dockerfileinstructions.Parse(result.AST)
	if err != nil {
		return err
	}
	return nil
}
