package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func CreateWorkflowV3(user string, workflowModel *commonmodels.WorkflowV3, logger *zap.SugaredLogger) error {
	if !checkWorkflowSubModules(workflowModel) {
		errStr := "工作流没有子模块，请先设置子模块"
		return e.ErrUpsertWorkflow.AddDesc(errStr)
	}
	if err := ensureWorkflowV3(workflowModel, logger); err != nil {
		return e.ErrUpsertWorkflow.AddDesc(err.Error())
	}

	workflowModel.CreatedBy = user
	workflowModel.UpdatedBy = user
	workflowModel.CreateTime = time.Now().Unix()
	workflowModel.UpdateTime = time.Now().Unix()

	if err := commonrepo.NewWorkflowV3Coll().Create(workflowModel); err != nil {
		logger.Errorf("Failed to create workflow v3, the error is: %s", err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	return nil
}

func ensureWorkflowV3(args *commonmodels.WorkflowV3, log *zap.SugaredLogger) error {
	if !defaultNameRegex.MatchString(args.Name) {
		log.Errorf("workflow name must match %s", defaultNameRegexString)
		return fmt.Errorf("%s %s", e.InvalidFormatErrMsg, defaultNameRegexString)
	}

	if err := validateV3SubTaskSetting(args.Name, args.SubTasks); err != nil {
		log.Errorf("validateV3SubTaskSetting: %+v", err)
		return err
	}

	if workflowV3, err := commonrepo.NewWorkflowV3Coll().Find(args.Name); err == nil {
		errStr := fmt.Sprintf("workflow [%s] 在项目 [%s] 中已经存在!", workflowV3.Name, workflowV3.ProjectName)
		return e.ErrCreatePipeline.AddDesc(errStr)
	}

	if workflow, err := commonrepo.NewWorkflowColl().Find(args.Name); err == nil {
		errStr := fmt.Sprintf("workflow [%s] 在项目 [%s] 中已经存在!", workflow.Name, workflow.ProductTmplName)
		return e.ErrCreatePipeline.AddDesc(errStr)
	}
	return nil
}

// validateV3SubTaskSetting Validating subtasks
func validateV3SubTaskSetting(workflowName string, subtasks []map[string]interface{}) error {
	for i, subTask := range subtasks {
		pre, err := base.ToPreview(subTask)
		if err != nil {
			return errors.New(e.InterfaceToTaskErrMsg)
		}

		if !pre.Enabled {
			continue
		}

		switch pre.TaskType {
		case config.TaskBuildV3:
			t, err := base.ToBuildTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}
			// 设置默认 build OS 为 Ubuntu 16.04
			if t.BuildOS == "" {
				t.BuildOS = setting.UbuntuXenial
				t.ImageFrom = commonmodels.ImageFromKoderover
			}

			ensureTaskSecretEnvs(workflowName, config.TaskBuildV3, t.JobCtx.EnvVars)

			subtasks[i], err = t.ToSubTask()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func checkWorkflowSubModules(args *commonmodels.WorkflowV3) bool {
	if args.SubTasks != nil && len(args.SubTasks) > 0 {
		return true
	}

	return false
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

func UpdateWorkflowV3(id, user string, workflowModel *commonmodels.WorkflowV3, logger *zap.SugaredLogger) error {
	if !checkWorkflowSubModules(workflowModel) {
		errStr := "工作流没有子模块，请先设置子模块"
		return e.ErrCreatePipeline.AddDesc(errStr)
	}
	workflowModel.UpdatedBy = user
	workflowModel.UpdateTime = time.Now().Unix()
	err := commonrepo.NewWorkflowV3Coll().Update(
		id,
		workflowModel,
	)
	if err != nil {
		logger.Errorf("update workflowV3 error: %s", err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	return nil
}

func DeleteWorkflowV3(id string, logger *zap.SugaredLogger) error {
	err := commonrepo.NewWorkflowV3Coll().DeleteByID(id)
	if err != nil {
		logger.Errorf("Failed to WorkflowV3 of id: %s, the error is: %s", id, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}
	return nil
}
