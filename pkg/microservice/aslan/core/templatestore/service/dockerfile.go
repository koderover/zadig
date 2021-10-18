package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/templatestore/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/templatestore/repository/mongodb"
)

func CreateDockerfileTemplate(template *DockerfileTemplate, logger *zap.SugaredLogger) error {
	err := mongodb.NewDockerfileTemplateColl().Create(&models.DockerfileTemplate{
		Name:    template.Name,
		Content: template.Content,
	})
	if err != nil {
		logger.Errorf("create dockerfile template error: %+v", err)
	}
	return err
}

func UpdateDockerfileTemplate(id string, template *DockerfileTemplate, logger *zap.SugaredLogger) error {
	err := mongodb.NewDockerfileTemplateColl().Update(
		id,
		&models.DockerfileTemplate{
			Name:    template.Name,
			Content: template.Content,
		},
	)
	if err != nil {
		logger.Errorf("update dockerfile template error: %+v", err)
	}
	return err
}

func ListDockerfileTemplate(pageNum, pageSize int, logger *zap.SugaredLogger) ([]*DockerfileListObject, int, error) {
	resp := make([]*DockerfileListObject, 0)
	templateList, total, err := mongodb.NewDockerfileTemplateColl().List(pageNum, pageSize)
	if err != nil {
		logger.Errorf("list dockerfile template error: %+v", err)
	}
	for _, obj := range templateList {
		resp = append(resp, &DockerfileListObject{
			ID:   obj.ID.String(),
			Name: obj.Name,
		})
	}
	return resp, total, err
}

func GetDockerfileTemplateDetail(id string, logger *zap.SugaredLogger) (*DockerfileDetail, error) {
	resp := new(DockerfileDetail)
	dockerfileTemplate, err := mongodb.NewDockerfileTemplateColl().GetById(id)
	if err != nil {
		logger.Errorf("Failed to get dockerfile template from id: %s, the error is: %+v", id, err)
		return nil, err
	}
	variables := getVariables(dockerfileTemplate.Content)
	resp.ID = dockerfileTemplate.ID.String()
	resp.Name = dockerfileTemplate.Name
	resp.Content = dockerfileTemplate.Content
	resp.Variables = variables
	return resp, nil
}

func DeleteDockerfileTemplate(id string, logger *zap.SugaredLogger) error {
	err := mongodb.NewDockerfileTemplateColl().DeleteByID(id)
	if err != nil {
		logger.Errorf("Failed to delete dockerfile template of id: %s, the error is: %+v", id, err)
	}
	return err
}

func GetDockerfileTemplateReference(id string, logger *zap.SugaredLogger) ([]*DockerfileDetail, error) {
	return []*DockerfileDetail{}, nil
}

func getVariables(s string) []*Variable {
	return []*Variable{}
}
