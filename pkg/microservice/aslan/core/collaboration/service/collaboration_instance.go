package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/repository/mongodb"
)

func GetCollaborationUpdate(projectName, uid string, logger *zap.SugaredLogger) (*GetCollaborationModeResp, error) {
	collaborations, err := mongodb.NewCollaborationModeColl().List(&mongodb.CollaborationModeFindOptions{
		ProjectName: projectName,
	})
	if err != nil {
		logger.Errorf("UpdateCollaborationMode error, err msg:%s", err)
		return nil, err
	}
	return &GetCollaborationModeResp{
		Collaborations: collaborations,
	}, nil
}
