package workflow

import (
	"encoding/json"
	"time"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
)

func CreateWorkflowV3(user string, workflowModel *commonmodels.WorkflowV3, logger *zap.SugaredLogger) error {
	logger.Infof("workflowModel:%+v", workflowModel)
	workflowModel.CreatedBy = user
	workflowModel.UpdatedBy = user
	workflowModel.CreateTime = time.Now().Unix()
	workflowModel.UpdateTime = time.Now().Unix()
	return commonrepo.NewWorkflowV3Coll().Create(workflowModel)
}

func ListWorkflowsV3(projectName string, pageNum, pageSize int64, logger *zap.SugaredLogger) ([]*WorkflowV3Brief, int64, error) {
	resp := make([]*WorkflowV3Brief, 0)
	workflowV3List, total, err := commonrepo.NewWorkflowV3Coll().List(&commonrepo.ListWorkflowV3Option{
		ProjectName: projectName,
	}, pageNum, pageSize)
	if err != nil {
		logger.Errorf("Failed to list workflow v3, the error is: %s", err)
		return nil, 0, err
	}
	for _, workflow := range workflowV3List {
		resp = append(resp, &WorkflowV3Brief{
			ID:          workflow.ID.Hex(),
			Name:        workflow.Name,
			ProjectName: workflow.ProjectName,
		})
	}
	return resp, total, nil
}

func GetWorkflowV3Detail(id string, logger *zap.SugaredLogger) (*WorkflowV3, error) {
	resp := new(WorkflowV3)
	workflow, err := commonrepo.NewWorkflowV3Coll().GetByID(id)
	if err != nil {
		logger.Errorf("Failed to get workflowV3 detail from id: %s, the error is: %s", id, err)
		return nil, err
	}
	out, err := json.Marshal(workflow)
	if err != nil {
		logger.Errorf("Failed to unmarshal given workflow, the error is: %s", err)
		return nil, err
	}
	err = json.Unmarshal(out, &resp)
	if err != nil {
		logger.Errorf("Cannot convert workflow into database model, the error is: %s", err)
		return nil, err
	}
	return resp, nil
}

func UpdateWorkflowV3(id, user string, workflow *WorkflowV3, logger *zap.SugaredLogger) error {
	workflowModel := new(commonmodels.WorkflowV3)
	out, err := json.Marshal(workflow)
	if err != nil {
		logger.Errorf("Failed to unmarshal given workflow, the error is: %s", err)
		return err
	}
	err = json.Unmarshal(out, &workflowModel)
	if err != nil {
		logger.Errorf("Cannot convert workflow into database model, the error is: %s", err)
		return err
	}
	workflowModel.UpdatedBy = user
	workflowModel.UpdateTime = time.Now().Unix()
	err = commonrepo.NewWorkflowV3Coll().Update(
		id,
		workflowModel,
	)
	if err != nil {
		logger.Errorf("update workflowV3 error: %s", err)
	}
	return err
}

func DeleteWorkflowV3(id string, logger *zap.SugaredLogger) error {
	err := commonrepo.NewWorkflowV3Coll().DeleteByID(id)
	if err != nil {
		logger.Errorf("Failed to WorkflowV3 of id: %s, the error is: %s", id, err)
	}
	return err
}
