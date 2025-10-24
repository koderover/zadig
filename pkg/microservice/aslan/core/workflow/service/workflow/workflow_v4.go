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

package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pingcap/tidb/parser"
	_ "github.com/pingcap/tidb/parser/test_driver"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"gorm.io/gorm/utils"
	"helm.sh/helm/v3/pkg/releaseutil"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/utils/pointer"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/msg_queue"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/collaboration"
	helmservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/helm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	larkservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	commomtemplate "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"github.com/koderover/zadig/v2/pkg/microservice/picket/client/opa"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/jenkins"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/types"
	generalutil "github.com/koderover/zadig/v2/pkg/util"
)

func CreateWorkflowV4(user string, workflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	existedWorkflow, err := commonrepo.NewWorkflowV4Coll().Find(workflow.Name)
	if err == nil {
		errStr := fmt.Sprintf("与项目 [%s] 中的工作流 [%s] 标识相同", existedWorkflow.Project, existedWorkflow.DisplayName)
		return e.ErrUpsertWorkflow.AddDesc(errStr)
	}

	workflowController := controller.CreateWorkflowController(workflow)
	if err := workflowController.Validate(false); err != nil {
		return err
	}

	workflow.CreatedBy = user
	workflow.UpdatedBy = user
	workflow.CreateTime = time.Now().Unix()
	workflow.UpdateTime = time.Now().Unix()

	if _, err := commonrepo.NewWorkflowV4Coll().Create(workflow); err != nil {
		logger.Errorf("Failed to create workflow v4, the error is: %s", err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}

	savedWorkflow, err := FindWorkflowV4("", workflow.Name, logger)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflow.Name, err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}

	err = UpdateWorkflowV4(savedWorkflow.Name, user, savedWorkflow, logger)
	if err != nil {
		logger.Errorf("update workflowV4 error: %s", err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}

	return nil
}

func SetWorkflowTasksCustomFields(projectName, workflowName string, args *models.CustomField, logger *zap.SugaredLogger) error {
	err := commonrepo.NewWorkflowV4Coll().SetCustomFields(commonrepo.SetCustomFieldsOptions{
		ProjectName:  projectName,
		WorkflowName: workflowName,
	}, args)
	if err != nil {
		logger.Errorf("Failed to set project: %s workflowV4: %s custom fields: %v, the error is: %s", projectName, workflowName, args, err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	return nil
}

func GetWorkflowTasksCustomFields(projectName, workflowName string, logger *zap.SugaredLogger) (*models.CustomField, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().FindByOptions(commonrepo.WorkFlowOptions{
		ProjectName:  projectName,
		WorkflowName: workflowName,
	})
	if err != nil {
		logger.Errorf("Failed to get project: %s workflowV4: %s infos, the error is: %s", projectName, workflowName, err)
		return nil, e.ErrUpsertWorkflow.AddErr(err)
	}

	fields := workflow.CustomField
	if fields == nil {
		fields = &models.CustomField{
			TaskID:                 1,
			Status:                 1,
			Duration:               1,
			Executor:               1,
			Remark:                 0,
			Branch:                 0,
			BuildServiceComponent:  make(map[string]int),
			BuildCodeMsg:           make(map[string]int),
			DeployServiceComponent: make(map[string]int),
			DeployEnv:              make(map[string]int),
			TestResult:             make(map[string]int),
		}
	}

	buildJobNames, err := commonrepo.NewWorkflowV4Coll().GetJobNameList(projectName, workflowName, string(config.JobZadigBuild))
	if err != nil {
		if err != mongo.ErrNoDocuments && err != mongo.ErrNilDocument {
			return nil, err
		}
	}
	if fields.BuildServiceComponent == nil {
		fields.BuildServiceComponent = make(map[string]int)
	}
	for key := range fields.BuildServiceComponent {
		if !utils.Contains(buildJobNames, key) {
			delete(fields.BuildServiceComponent, key)
		}
	}

	if fields.BuildCodeMsg == nil {
		fields.BuildCodeMsg = make(map[string]int)
	}
	for key := range fields.BuildCodeMsg {
		if !utils.Contains(buildJobNames, key) {
			delete(fields.BuildCodeMsg, key)
		}
	}
	for _, jobName := range buildJobNames {
		if _, ok := fields.BuildServiceComponent[jobName]; !ok {
			fields.BuildServiceComponent[jobName] = 0
		}
		if _, ok := fields.BuildCodeMsg[jobName]; !ok {
			fields.BuildCodeMsg[jobName] = 0
		}
	}

	deployJobNames, err := commonrepo.NewWorkflowV4Coll().GetJobNameList(projectName, workflowName, string(config.JobZadigDeploy))
	if err != nil {
		if err != mongo.ErrNoDocuments && err != mongo.ErrNilDocument {
			return nil, err
		}
	}

	if fields.DeployServiceComponent == nil {
		fields.DeployServiceComponent = make(map[string]int)
	}
	for key := range fields.DeployServiceComponent {
		if !utils.Contains(deployJobNames, key) {
			delete(fields.DeployServiceComponent, key)
		}
	}

	if fields.DeployEnv == nil {
		fields.DeployEnv = make(map[string]int)
	}
	for key := range fields.DeployEnv {
		if !utils.Contains(deployJobNames, key) {
			delete(fields.DeployEnv, key)
		}
	}

	for _, jobName := range deployJobNames {
		if _, ok := fields.DeployServiceComponent[jobName]; !ok {
			fields.DeployServiceComponent[jobName] = 0
		}
		if _, ok := fields.DeployEnv[jobName]; !ok {
			fields.DeployEnv[jobName] = 0
		}
	}

	testJobNames, err := commonrepo.NewWorkflowV4Coll().GetJobNameList(projectName, workflowName, string(config.JobZadigTesting))
	if err != nil {
		if err != mongo.ErrNoDocuments && err != mongo.ErrNilDocument {
			return nil, err
		}
	}
	if fields.TestResult == nil {
		fields.TestResult = make(map[string]int)
	}
	for key := range fields.TestResult {
		if !utils.Contains(testJobNames, key) {
			delete(fields.TestResult, key)
		}
	}
	for _, jobName := range testJobNames {
		if _, ok := fields.TestResult[jobName]; !ok {
			fields.TestResult[jobName] = 0
		}
	}
	return fields, nil
}

func UpdateWorkflowV4(name, user string, inputWorkflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(name)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", name, err)
		return e.ErrFindWorkflow.AddErr(err)
	}
	if workflow.DisplayName != inputWorkflow.DisplayName {
		existedWorkflows, _, _ := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{ProjectName: workflow.Project, DisplayName: inputWorkflow.DisplayName}, 0, 0)
		if len(existedWorkflows) > 0 {
			errStr := fmt.Sprintf("workflow v4 [%s] 展示名称在当前项目下重复!", inputWorkflow.DisplayName)
			return e.ErrUpsertWorkflow.AddDesc(errStr)
		}
	}

	workflowController := controller.CreateWorkflowController(inputWorkflow)
	if err := workflowController.Validate(false); err != nil {
		return err
	}

	inputWorkflow.UpdatedBy = user
	inputWorkflow.UpdateTime = time.Now().Unix()
	inputWorkflow.ID = workflow.ID
	inputWorkflow.CustomField = workflow.CustomField

	if err := commonrepo.NewWorkflowV4Coll().Update(
		workflow.ID.Hex(),
		inputWorkflow,
	); err != nil {
		logger.Errorf("update workflowV4 error: %s", err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	return nil
}

func FindWorkflowV4(encryptedKey, name string, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(name)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", name, err)
		return workflow, e.ErrFindWorkflow.AddErr(err)
	}

	if err := ensureWorkflowV4Resp(encryptedKey, workflow, logger); err != nil {
		return workflow, err
	}

	return workflow, err
}

func FindWorkflowV4Raw(name string, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(name)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", name, err)
		return workflow, e.ErrFindWorkflow.AddErr(err)
	}
	return workflow, err
}

func DeleteWorkflowV4(name string, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(name)
	if err != nil {
		logger.Errorf("Failed to delete WorkflowV4: %s, the error is: %v", name, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}
	if err := commonrepo.NewWorkflowV4Coll().DeleteByID(workflow.ID.Hex()); err != nil {
		logger.Errorf("Failed to delete WorkflowV4: %s, the error is: %v", name, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}
	if err := commonrepo.NewworkflowTaskv4Coll().DeleteByWorkflowName(name); err != nil {
		logger.Errorf("Failed to delete WorkflowV4 task: %s, the error is: %v", name, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}
	if err := commonrepo.NewCounterColl().Delete("WorkflowTaskV4:" + name); err != nil {
		log.Errorf("Counter.Delete error: %s", err)
	}
	return nil
}

func ListWorkflowV4(projectName, viewName, userID string, names, v4Names []string, policyFound bool, logger *zap.SugaredLogger) ([]*Workflow, error) {
	resp := make([]*Workflow, 0)
	var err error
	//ignoreWorkflow := false
	ignoreWorkflowV4 := false
	if viewName == "" {
		//if policyFound && len(names) == 0 {
		//	ignoreWorkflow = true
		//}
		if policyFound && len(v4Names) == 0 {
			ignoreWorkflowV4 = true
		}
	} else {
		names, v4Names, err = filterWorkflowNamesByView(projectName, viewName, names, v4Names, policyFound)
		if err != nil {
			logger.Errorf("filterWorkflowNames error: %s", err)
			return resp, err
		}
		//if len(names) == 0 {
		//	ignoreWorkflow = true
		//}
		if len(v4Names) == 0 {
			ignoreWorkflowV4 = true
		}
	}
	workflowV4List := []*commonmodels.WorkflowV4{}
	if !ignoreWorkflowV4 {
		workflowV4List, _, err = commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{
			ProjectName: projectName,
			Names:       v4Names,
		}, 0, 0)
		if err != nil {
			logger.Errorf("Failed to list workflow v4, the error is: %s", err)
			return resp, err
		}
	}

	workflow := []*Workflow{}

	workflowList := []string{}
	for _, wV4 := range workflowV4List {
		workflowList = append(workflowList, wV4.Name)
	}
	resp = append(resp, workflow...)
	workflowCMMap, err := collaboration.GetWorkflowCMMap([]string{projectName}, logger)
	if err != nil {
		return nil, err
	}

	tasks, err := getRecentWorkflowTask(workflowList)
	if err != nil {
		return nil, err
	}

	favorites, err := commonrepo.NewFavoriteColl().List(&commonrepo.FavoriteArgs{UserID: userID, Type: string(config.WorkflowTypeV4)})
	if err != nil {
		return resp, errors.Errorf("failed to get custom workflow favorite data, err: %v", err)
	}
	favoriteSet := sets.NewString()
	for _, f := range favorites {
		favoriteSet.Insert(f.Name)
	}
	workflowStatMap := getWorkflowStatMap(workflowList, config.WorkflowTypeV4)

	for _, workflowModel := range workflowV4List {
		stages := []string{}
		for _, stage := range workflowModel.Stages {
			stages = append(stages, stage.Name)
		}
		var baseRefs []string
		if cmSet, ok := workflowCMMap[collaboration.BuildWorkflowCMMapKey(workflowModel.Project, workflowModel.Name)]; ok {
			for _, cm := range cmSet.List() {
				baseRefs = append(baseRefs, cm)
			}
		}
		workflow := &Workflow{
			Name:                 workflowModel.Name,
			DisplayName:          workflowModel.DisplayName,
			ProjectName:          workflowModel.Project,
			Disabled:             workflowModel.Disabled,
			EnabledStages:        stages,
			CreateTime:           workflowModel.CreateTime,
			UpdateTime:           workflowModel.UpdateTime,
			UpdateBy:             workflowModel.UpdatedBy,
			WorkflowType:         setting.CustomWorkflowType,
			Description:          workflowModel.Description,
			BaseRefs:             baseRefs,
			BaseName:             workflowModel.BaseName,
			EnableApprovalTicket: workflowModel.EnableApprovalTicket,
		}
		if workflowModel.Category == setting.ReleaseWorkflow {
			workflow.WorkflowType = string(setting.ReleaseWorkflow)
		}
		if favoriteSet.Has(workflow.Name) {
			workflow.IsFavorite = true
		}
		setRecentTaskV4Info(workflow, tasks)
		setWorkflowStat(workflow, workflowStatMap)

		resp = append(resp, workflow)
	}
	return resp, nil
}

type WorkflowWithAction struct {
	WorkflowName string
	Action       user.WorkflowActions
}

type ProjectAuthWorkflow struct {
	ProjectName              string
	IsProjectAdmin           bool
	Actions                  *user.WorkflowActions
	CollModeWorkflowPermsMap map[string]*WorkflowWithAction
}

type ListGlobalWorkflowV4Query struct {
	ProjectName    string
	IsFavorite     bool
	Keyword        string
	ProjectAuthMap map[string]*ProjectAuthWorkflow
	PageNum        int64
	PageSize       int64
	SortBy         setting.ListWorkflowV4InGlobalSortBy
	OrderBy        setting.ListWorkflowV4InGlobalOrderBy
}

type ListGlobalWorkflowV4Response struct {
	WorkflowList []*WorkflowWithActions `json:"workflow_list"`
	Total        int64                  `json:"total"`
}

type WorkflowWithActions struct {
	Workflow *Workflow             `json:"workflow"`
	Actions  *user.WorkflowActions `json:"actions"`
}

func ListWorkflowV4InGlobal(ctx *internalhandler.Context, query *ListGlobalWorkflowV4Query) (*ListGlobalWorkflowV4Response, error) {
	var (
		err                      error
		total                    int64
		workflowModels           []*commonmodels.WorkflowV4
		favoriteWorkflowSet      = sets.NewString()
		favoriteWorkflowQueryArg = []string{}
	)

	favorites, err := commonrepo.NewFavoriteColl().List(&commonrepo.FavoriteArgs{
		UserID:      ctx.UserID,
		Type:        string(config.WorkflowTypeV4),
		ProductName: query.ProjectName,
	})
	if err != nil {
		return nil, errors.Errorf("failed to get workflow favorite data, err: %v", err)
	}

	for _, f := range favorites {
		favoriteWorkflowSet.Insert(f.Name)
	}

	if query.IsFavorite {
		if favoriteWorkflowSet.Len() == 0 {
			return &ListGlobalWorkflowV4Response{
				WorkflowList: make([]*WorkflowWithActions, 0),
				Total:        0,
			}, nil
		}
		favoriteWorkflowQueryArg = favoriteWorkflowSet.List()
	}

	if ctx.Resources.IsSystemAdmin {
		workflowModels, total, err = commonrepo.NewWorkflowV4Coll().ListInGlobal(&commonrepo.ListWorkflowV4InGlobalOption{
			ProjectName:           query.ProjectName,
			Keyword:               query.Keyword,
			FavoriteWorkflowNames: favoriteWorkflowQueryArg,
			SortBy:                query.SortBy,
			OrderBy:               query.OrderBy,
			PageNum:               query.PageNum,
			PageSize:              query.PageSize,
		})
		if err != nil {
			return nil, errors.Errorf("failed to list workflow v4, err: %v", err)
		}
	} else {
		projectNames := []string{}
		collModeWorkflowNames := []string{}
		for _, projectAuth := range query.ProjectAuthMap {
			if projectAuth.IsProjectAdmin || projectAuth.Actions.View {
				projectNames = append(projectNames, projectAuth.ProjectName)
			} else {
				for _, collModeWorkflowPerm := range projectAuth.CollModeWorkflowPermsMap {
					collModeWorkflowNames = append(collModeWorkflowNames, collModeWorkflowPerm.WorkflowName)
				}
			}
		}

		workflowModels, total, err = commonrepo.NewWorkflowV4Coll().ListInGlobal(&commonrepo.ListWorkflowV4InGlobalOption{
			ProjectName:           query.ProjectName,
			Keyword:               query.Keyword,
			ProjectNames:          projectNames,
			CollModeWorkflowNames: collModeWorkflowNames,
			FavoriteWorkflowNames: favoriteWorkflowQueryArg,
			SortBy:                query.SortBy,
			OrderBy:               query.OrderBy,
			PageNum:               query.PageNum,
			PageSize:              query.PageSize,
		})
		if err != nil {
			return nil, errors.Errorf("failed to list workflow v4, err: %v", err)
		}
	}

	workflowNames := []string{}
	for _, workflowModel := range workflowModels {
		workflowNames = append(workflowNames, workflowModel.Name)
	}

	workflowTasks, err := getRecentWorkflowTask(workflowNames)
	if err != nil {
		return nil, err
	}
	workflowStatMap := getWorkflowStatMap(workflowNames, config.WorkflowTypeV4)

	workflows := make([]*Workflow, 0)
	for _, workflowModel := range workflowModels {
		stages := []string{}
		for _, stage := range workflowModel.Stages {
			stages = append(stages, stage.Name)
		}

		workflow := &Workflow{
			Name:                 workflowModel.Name,
			DisplayName:          workflowModel.DisplayName,
			ProjectName:          workflowModel.Project,
			Disabled:             workflowModel.Disabled,
			IsFavorite:           favoriteWorkflowSet.Has(workflowModel.Name),
			EnabledStages:        stages,
			CreateTime:           workflowModel.CreateTime,
			UpdateTime:           workflowModel.UpdateTime,
			UpdateBy:             workflowModel.UpdatedBy,
			WorkflowType:         setting.CustomWorkflowType,
			Description:          workflowModel.Description,
			BaseName:             workflowModel.BaseName,
			EnableApprovalTicket: workflowModel.EnableApprovalTicket,
		}

		setRecentTaskV4Info(workflow, workflowTasks)
		setWorkflowStat(workflow, workflowStatMap)

		workflows = append(workflows, workflow)
	}

	WorkflowWithActionsList := genWorkflowActionsList(ctx, workflows, query.ProjectAuthMap)
	return &ListGlobalWorkflowV4Response{
		WorkflowList: WorkflowWithActionsList,
		Total:        total,
	}, nil
}

func getRecentWorkflowTask(workflowNames []string) ([]*commonmodels.WorkflowTask, error) {
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		err   error
		tasks []*models.WorkflowTask
	)
	for _, name := range workflowNames {
		wg.Add(1)
		go func(workflowName string) {
			defer wg.Done()
			resp, _, err2 := commonrepo.NewworkflowTaskv4Coll().List(&commonrepo.ListWorkflowTaskV4Option{
				WorkflowName: workflowName,
				Limit:        10,
			})
			if err2 != nil {
				err = err2
				return
			}
			mu.Lock()
			defer mu.Unlock()
			tasks = append(tasks, resp...)
		}(name)
	}
	wg.Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow tasks, err: %v", err)
	}
	return tasks, nil
}

func genWorkflowActionsList(ctx *internalhandler.Context, workflows []*Workflow, projectAuthMap map[string]*ProjectAuthWorkflow) []*WorkflowWithActions {
	WorkflowWithActionsList := make([]*WorkflowWithActions, 0)
	for _, workflow := range workflows {
		actions := &user.WorkflowActions{}
		if ctx.Resources.IsSystemAdmin {
			actions.Create = true
			actions.View = true
			actions.Edit = true
			actions.Delete = true
			actions.Execute = true
			actions.Debug = true
		} else if projectAuth, ok := projectAuthMap[workflow.ProjectName]; ok {
			if projectAuth.IsProjectAdmin {
				actions.Create = true
				actions.View = true
				actions.Edit = true
				actions.Delete = true
				actions.Execute = true
				actions.Debug = true
			} else {
				actions.Create = projectAuth.Actions.Create
				actions.View = projectAuth.Actions.View
				actions.Edit = projectAuth.Actions.Edit
				actions.Delete = projectAuth.Actions.Delete
				actions.Execute = projectAuth.Actions.Execute
				actions.Debug = projectAuth.Actions.Debug
			}

			// override with collaboration mode permissions
			if projectAuth.CollModeWorkflowPermsMap != nil {
				if collModeWorkflowPerm, ok := projectAuth.CollModeWorkflowPermsMap[workflow.Name]; ok {
					if collModeWorkflowPerm.Action.Create {
						actions.Create = true
					}
					if collModeWorkflowPerm.Action.View {
						actions.View = true
					}
					if collModeWorkflowPerm.Action.Edit {
						actions.Edit = true
					}
					if collModeWorkflowPerm.Action.Execute {
						actions.Execute = true
					}
					if collModeWorkflowPerm.Action.Debug {
						actions.Debug = true
					}
				}
			}
		}

		workflowWithActions := &WorkflowWithActions{
			Workflow: workflow,
			Actions:  actions,
		}

		WorkflowWithActionsList = append(WorkflowWithActionsList, workflowWithActions)
	}

	return WorkflowWithActionsList
}

type NameWithParams struct {
	Name        string                `json:"name"`
	DisplayName string                `json:"display_name"`
	ProjectName string                `json:"project_name"`
	Params      []*commonmodels.Param `json:"params"`
}

func ListWorkflowV4CanTrigger(ctx *internalhandler.Context) ([]*NameWithParams, error) {
	var workflowList []*models.WorkflowV4
	var err error

	if ctx.Resources.IsSystemAdmin {
		// if a user is system admin, we simply list all custom workflows
		workflowList, _, err = commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{}, 0, 0)
		if err != nil {
			ctx.Logger.Errorf("Failed to list all custom workflows from system, err: %s", err)
			return nil, errors.Errorf("failed to list workflow v4, the error is: %s", err)
		}
	} else {
		projects, _, projectFindErr := user.New().ListAuthorizedProjects(ctx.UserID)
		if projectFindErr != nil {
			ctx.Logger.Errorf("failed to list authorized project for normal user, err: %s", projectFindErr)
			return nil, errors.New("failed to get allowed projects")
		}

		for _, authorizedProject := range projects {
			// since it is in authorized project list, it either has a role in this project, or it is in a collaboration mode.
			if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[authorizedProject]; ok {
				// if it has a project role, check it
				if projectAuthInfo.IsProjectAdmin || projectAuthInfo.Workflow.View {
					projectedWorkflowList, _, workflowFindErr := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{
						ProjectName: authorizedProject,
					}, 0, 0)
					if workflowFindErr != nil {
						ctx.Logger.Errorf("failed to find authorized workflows for project: %s, error is: %s", authorizedProject, workflowFindErr)
						return nil, errors.New("failed to get allowed workflows")
					}
					workflowList = append(workflowList, projectedWorkflowList...)
				} else {
					// if it does not have any kind of authz from role, we check from collaboration mode anyway.
					authorizedProjectedCustomWorkflow, findErr := getAuthorizedCustomWorkflowFromCollaborationMode(ctx, authorizedProject)
					if findErr != nil {
						ctx.Logger.Errorf("failed to find authorized workflows for project: %s, error is: %s", authorizedProject, findErr)
						return nil, errors.New("failed to get allowed workflows")
					}
					workflowList = append(workflowList, authorizedProjectedCustomWorkflow...)
				}
			} else {
				// otherwise simply check for collaboration mode will suffice
				authorizedProjectedCustomWorkflow, findErr := getAuthorizedCustomWorkflowFromCollaborationMode(ctx, authorizedProject)
				if findErr != nil {
					ctx.Logger.Errorf("failed to find authorized workflows for project: %s, error is: %s", authorizedProject, findErr)
					return nil, errors.New("failed to get allowed workflows")
				}
				workflowList = append(workflowList, authorizedProjectedCustomWorkflow...)
			}

		}
	}

	var result []*NameWithParams
LOOP:
	for _, workflowV4 := range workflowList {
		for _, stage := range workflowV4.Stages {
			for _, job := range stage.Jobs {
				switch job.JobType {
				case config.JobFreestyle:
					spec := new(commonmodels.FreestyleJobSpec)
					err = models.IToi(stage.Jobs[0].Spec, spec)
					if err != nil {
						return nil, fmt.Errorf("faield to decode spec for freestyle job %s in workflow %s, error: %s", job.Name, workflowV4.Name, err)
					}
					if spec.FreestyleJobType == config.ServiceFreeStyleJobType {
						continue LOOP
					}
				case config.JobPlugin, config.JobWorkflowTrigger:
				default:
					continue LOOP
				}
			}
		}
		var paramList []*commonmodels.Param
		for _, param := range workflowV4.Params {
			if !strings.Contains(param.Value, setting.FixedValueMark) {
				paramList = append(paramList, param)
			}
		}
		result = append(result, &NameWithParams{
			Name:        workflowV4.Name,
			DisplayName: workflowV4.DisplayName,
			ProjectName: workflowV4.Project,
			Params:      paramList,
		})
	}
	return result, nil
}

func AutoCreateWorkflow(productName string, log *zap.SugaredLogger) *EnvStatus {
	productTmpl, err := template.NewProductColl().Find(productName)
	if err != nil {
		errMsg := fmt.Sprintf("[ProductTmpl.Find] %s error: %v", productName, err)
		log.Error(errMsg)
		return &EnvStatus{Status: setting.ProductStatusFailed, ErrMessage: errMsg}
	}
	errList := new(multierror.Error)

	mut := cache.NewRedisLock(fmt.Sprintf("auto_create_product:%s", productName))

	mut.Lock()
	defer func() {
		mut.Unlock()
	}()

	systemSetting, err := commonrepo.NewSystemSettingColl().Get()
	if err != nil {
		errMsg := fmt.Sprintf("failed to get system setting, err: %s", err)
		log.Error(errMsg)
		return &EnvStatus{Status: setting.ProductStatusFailed, ErrMessage: errMsg}
	}

	createArgs := &workflowCreateArgs{
		productName: productName,
		argsMap:     make(map[string]*workflowCreateArg),
	}
	createArgs.initDefaultWorkflows()

	s3storageID := ""
	s3storage, err := commonrepo.NewS3StorageColl().FindDefault()
	if err != nil {
		log.Errorf("S3Storage.FindDefault error: %v", err)
	} else {
		projectSet := sets.NewString(s3storage.Projects...)
		if projectSet.Has(productName) || projectSet.Has(setting.AllProjects) {
			s3storageID = s3storage.ID.Hex()
		}
	}

	productList, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:       productName,
		Production: generalutil.GetBoolPointer(false),
	})
	if err != nil {
		log.Errorf("fialed to list products, projectName %s, err %s", productName, err)
	}

	createArgs.clear()
	for _, product := range productList {
		createArgs.addWorkflowArg(product.EnvName, product.RegistryID, true)
	}
	if !productTmpl.IsHostProduct() {
		createArgs.addWorkflowArg("", "", false)
	}

	workflowSet := sets.NewString()
	for workflowName := range createArgs.argsMap {
		_, err := FindWorkflowV4Raw(workflowName, log)
		if err == nil {
			workflowSet.Insert(workflowName)
		}
	}

	if len(workflowSet) < len(createArgs.argsMap) {
		services, err := commonrepo.NewServiceColl().ListMaxRevisionsForServices(productTmpl.AllTestServiceInfos(), "")
		if err != nil {
			log.Errorf("ServiceTmpl.ListMaxRevisionsByProject error: %v", err)
			errList = multierror.Append(errList, err)
		}
		buildList, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{
			ProductName: productName,
		})
		if err != nil {
			log.Errorf("[Build.List] error: %v", err)
			errList = multierror.Append(errList, err)
		}
		buildMap := map[string]*commonmodels.Build{}
		for _, build := range buildList {
			for _, target := range build.Targets {
				buildMap[target.ServiceName] = build
			}
		}

		for workflowName, workflowArg := range createArgs.argsMap {
			if workflowSet.Has(workflowName) {
				continue
			}
			if dupWorkflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName); err == nil {
				errList = multierror.Append(errList, fmt.Errorf("workflow [%s] 在项目 [%s] 中已经存在", workflowName, dupWorkflow.Project))
			}
			workflow := new(commonmodels.WorkflowV4)
			workflow.Project = productName
			workflow.Name = workflowName
			workflow.DisplayName = workflowName
			workflow.CreatedBy = setting.SystemUser
			workflow.UpdatedBy = setting.SystemUser
			workflow.CreateTime = time.Now().Unix()
			workflow.UpdateTime = time.Now().Unix()
			workflow.ConcurrencyLimit = 1

			buildJobName := ""
			if workflowArg.buildStageEnabled {
				buildTargetSet := sets.NewString()
				serviceAndBuilds := []*commonmodels.ServiceAndBuild{}
				for _, serviceTmpl := range services {
					if build, ok := buildMap[serviceTmpl.ServiceName]; ok {
						for _, target := range build.Targets {
							key := fmt.Sprintf("%s-%s", target.ServiceName, target.ServiceModule)
							if buildTargetSet.Has(key) {
								continue
							}
							buildTargetSet.Insert(key)

							serviceAndBuild := &commonmodels.ServiceAndBuild{
								ServiceName:   target.ServiceName,
								ServiceModule: target.ServiceModule,
								BuildName:     build.Name,
							}
							serviceAndBuilds = append(serviceAndBuilds, serviceAndBuild)
						}
					}
				}

				buildJobName = "构建"
				if systemSetting.Language == string(config.SystemLanguageEnUS) {
					buildJobName = "build"
				}
				buildJob := &commonmodels.Job{
					Name:    buildJobName,
					JobType: config.JobZadigBuild,
					Spec: &commonmodels.ZadigBuildJobSpec{
						DockerRegistryID:        workflowArg.dockerRegistryID,
						ServiceAndBuildsOptions: serviceAndBuilds,
					},
				}

				buildStageName := "构建"
				if systemSetting.Language == string(config.SystemLanguageEnUS) {
					buildStageName = "build"
				}
				stage := &commonmodels.WorkflowStage{
					Name:     buildStageName,
					Parallel: true,
					Jobs:     []*commonmodels.Job{buildJob},
				}
				workflow.Stages = append(workflow.Stages, stage)
			}

			if productTmpl.IsCVMProduct() {
				// logic move to generateHostCustomWorkflow function
				spec := &commonmodels.ZadigVMDeployJobSpec{
					Env:         workflowArg.envName,
					Source:      config.SourceRuntime,
					S3StorageID: s3storageID,
				}
				if workflowArg.buildStageEnabled {
					spec.Source = config.SourceFromJob
					spec.JobName = buildJobName
				}

				deployJobName := "主机部署"
				if systemSetting.Language == string(config.SystemLanguageEnUS) {
					deployJobName = "vm-deploy"
				}
				deployJob := &commonmodels.Job{
					Name:    deployJobName,
					JobType: config.JobZadigVMDeploy,
					Spec:    spec,
				}

				deployStageName := "主机部署"
				if systemSetting.Language == string(config.SystemLanguageEnUS) {
					deployStageName = "vm-deploy"
				}
				stage := &commonmodels.WorkflowStage{
					Name:     deployStageName,
					Parallel: true,
					Jobs:     []*commonmodels.Job{deployJob},
				}
				workflow.Stages = append(workflow.Stages, stage)
			} else {
				spec := &commonmodels.ZadigDeployJobSpec{
					Env: workflowArg.envName,
					DeployContents: []config.DeployContent{
						config.DeployImage,
					},
					Source:     config.SourceRuntime,
					Production: true,
				}
				if workflowArg.buildStageEnabled {
					spec.Source = config.SourceFromJob
					spec.JobName = buildJobName
					spec.Production = false
				}

				deployJobName := "部署"
				if systemSetting.Language == string(config.SystemLanguageEnUS) {
					deployJobName = "deploy"
				}
				deployJob := &commonmodels.Job{
					Name:    deployJobName,
					JobType: config.JobZadigDeploy,
					Spec:    spec,
				}

				deployStageName := "部署"
				if systemSetting.Language == string(config.SystemLanguageEnUS) {
					deployStageName = "deploy"
				}
				stage := &commonmodels.WorkflowStage{
					Name:     deployStageName,
					Parallel: true,
					Jobs:     []*commonmodels.Job{deployJob},
				}
				workflow.Stages = append(workflow.Stages, stage)
			}

			if _, err := commonrepo.NewWorkflowV4Coll().Create(workflow); err != nil {
				errList = multierror.Append(errList, err)
			}
		}
		if err = errList.ErrorOrNil(); err != nil {
			return &EnvStatus{Status: setting.ProductStatusFailed, ErrMessage: err.Error()}
		}
		return &EnvStatus{Status: setting.ProductStatusCreating}
	} else if len(workflowSet) == len(createArgs.argsMap) {
		return &EnvStatus{Status: setting.ProductStatusSuccess}
	}
	return nil
}

func GetArtifactFileContent(pipelineName string, taskID int64, notHistoryFileFlag bool, log *zap.SugaredLogger) ([]byte, error) {
	s3Storage, client, artifactFiles, artifactResultOutByts, err := getArtifactAndS3Info(pipelineName, "", taskID, notHistoryFileFlag, log)
	if err != nil {
		return nil, fmt.Errorf("download artifact err: %s", err)
	}
	if notHistoryFileFlag {
		return artifactResultOutByts, nil
	}
	tempDir, _ := ioutil.TempDir("", "")
	sourcePath := path.Join(tempDir, "artifact")
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		_ = os.MkdirAll(sourcePath, 0777)
	}

	for _, artifactFile := range artifactFiles {
		artifactFileArr := strings.Split(artifactFile, "/")
		if len(artifactFileArr) > 1 {
			artifactFileName := artifactFileArr[len(artifactFileArr)-1]
			file, err := os.Create(path.Join(sourcePath, artifactFileName))
			if err != nil {
				return nil, fmt.Errorf("failed to create file %s %v", artifactFileName, err)
			}
			defer func() {
				_ = file.Close()
			}()

			err = client.Download(s3Storage.Bucket, artifactFile, file.Name())
			if err != nil {
				return nil, fmt.Errorf("failed to download %s %v", artifactFile, err)
			}
		}
	}
	//将该目录压缩
	goCacheManager := new(GoCacheManager)
	artifactTarFileName := path.Join(sourcePath, "artifact.tar.gz")
	err = goCacheManager.Archive(sourcePath, artifactTarFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to Archive %s %v", sourcePath, err)
	}
	defer func() {
		_ = os.Remove(artifactTarFileName)
		_ = os.Remove(tempDir)
	}()

	fileBytes, err := ioutil.ReadFile(path.Join(sourcePath, "artifact.tar.gz"))
	return fileBytes, err
}

func getArtifactAndS3Info(pipelineName, dir string, taskID int64, notHistoryFileFlag bool, log *zap.SugaredLogger) (*s3.S3, *s3tool.Client, []string, []byte, error) {
	fis := make([]string, 0)

	storage, err := s3.FindDefaultS3()
	if err != nil {
		log.Errorf("GetTestArtifactInfo FindDefaultS3 err:%v", err)
		return nil, nil, fis, nil, err
	}

	if storage.Subfolder != "" {
		storage.Subfolder = fmt.Sprintf("%s/%s/%d/%s", storage.Subfolder, pipelineName, taskID, "artifact")
	} else {
		storage.Subfolder = fmt.Sprintf("%s/%d/%s", pipelineName, taskID, "artifact")
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Region, storage.Insecure, storage.Provider)
	if err != nil {
		log.Errorf("GetTestArtifactInfo Create S3 client err:%+v", err)
		return nil, nil, fis, nil, err
	}

	if notHistoryFileFlag {
		objectKey := storage.GetObjectPath(fmt.Sprintf("%s/%s/%s", dir, "workspace", setting.ArtifactResultOut))
		object, err := client.GetFile(storage.Bucket, objectKey, &s3tool.DownloadOption{RetryNum: 2})
		if err != nil {
			log.Errorf("GetTestArtifactInfo GetFile err:%s", err)
			return nil, nil, fis, nil, err
		}
		fileByts, err := ioutil.ReadAll(object.Body)
		if err != nil {
			log.Errorf("GetTestArtifactInfo ioutil.ReadAll err:%s", err)
			return nil, nil, fis, nil, err
		}
		return storage, client, fis, fileByts, nil
	}

	prefix := storage.GetObjectPath(dir)
	files, err := client.ListFiles(storage.Bucket, prefix, true)
	if err != nil || len(files) <= 0 {
		log.Errorf("GetTestArtifactInfo ListFiles err:%v", err)
		return nil, nil, fis, nil, err
	}
	return storage, client, files, nil, nil
}

func getAuthorizedCustomWorkflowFromCollaborationMode(ctx *internalhandler.Context, projectKey string) ([]*models.WorkflowV4, error) {
	_, authorizedCustomWorkflow, workflowFindErr := user.New().ListAuthorizedWorkflows(ctx.UserID, projectKey)
	if workflowFindErr != nil {
		ctx.Logger.Errorf("failed to find authorized workflows for project: %s, error is: %s", projectKey, workflowFindErr)
		return nil, errors.New("failed to get allowed workflows")
	}
	if len(authorizedCustomWorkflow) > 0 {
		// if there are authorized workflow, we get the details out of it
		projectedWorkflowList, _, workflowFindErr := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{
			ProjectName: projectKey,
			Names:       authorizedCustomWorkflow,
		}, 0, 0)
		if workflowFindErr != nil {
			ctx.Logger.Errorf("failed to find authorized workflows for project: %s, error is: %s", projectKey, workflowFindErr)
			return nil, errors.New("failed to get allowed workflows")
		}
		return projectedWorkflowList, nil
	}
	return make([]*models.WorkflowV4, 0), nil
}

func filterWorkflowNamesByView(projectName, viewName string, workflowNames, workflowV4Names []string, policyFound bool) ([]string, []string, error) {
	if viewName == "" {
		return workflowNames, workflowV4Names, nil
	}
	view, err := commonrepo.NewWorkflowViewColl().Find(projectName, viewName)
	if err != nil {
		return workflowNames, workflowV4Names, err
	}
	enabledWorkflow := []string{}
	enabledWorkflowV4 := []string{}
	for _, workflow := range view.Workflows {
		if !workflow.Enabled {
			continue
		}
		if workflow.WorkflowType == setting.CustomWorkflowType {
			enabledWorkflowV4 = append(enabledWorkflowV4, workflow.WorkflowName)
		} else {
			enabledWorkflow = append(enabledWorkflow, workflow.WorkflowName)
		}
	}
	if !policyFound {
		return enabledWorkflow, enabledWorkflowV4, nil
	}
	return intersection(workflowNames, enabledWorkflow), intersection(workflowV4Names, enabledWorkflowV4), nil
}

func intersection(a, b []string) []string {
	m := make(map[string]bool)
	var intersection []string
	for _, item := range a {
		m[item] = true
	}
	for _, item := range b {
		if _, ok := m[item]; ok {
			intersection = append(intersection, item)
		}
	}
	return intersection
}

func setRecentTaskV4Info(workflow *Workflow, tasks []*commonmodels.WorkflowTask) {
	recentTask := &commonmodels.WorkflowTask{}
	recentFailedTask := &commonmodels.WorkflowTask{}
	recentSucceedTask := &commonmodels.WorkflowTask{}
	workflow.NeverRun = true
	var workflowList []*commonmodels.WorkflowTask
	var recentTenTask []*commonmodels.WorkflowTask
	for _, task := range tasks {
		if task.WorkflowName != workflow.Name {
			continue
		}
		workflowList = append(workflowList, task)
		workflow.NeverRun = false
	}
	sort.Slice(workflowList, func(i, j int) bool {
		return workflowList[i].TaskID > workflowList[j].TaskID
	})
	for _, task := range workflowList {
		if recentSucceedTask.TaskID != 0 && recentFailedTask.TaskID != 0 && len(recentTenTask) == 10 {
			break
		}
		if recentTask.TaskID == 0 {
			recentTask = task
		}
		if task.Status == config.StatusPassed && recentSucceedTask.TaskID == 0 {
			recentSucceedTask = task
		}
		if task.Status == config.StatusFailed && recentFailedTask.TaskID == 0 {
			recentFailedTask = task
		}
		if len(recentTenTask) < 10 {
			recentTenTask = append(recentTenTask, task)
		}
	}
	if recentTask.TaskID > 0 {
		workflow.RecentTask = &TaskInfo{
			TaskID:       recentTask.TaskID,
			PipelineName: recentTask.WorkflowName,
			Status:       string(recentTask.Status),
			TaskCreator:  recentTask.TaskCreator,
			CreateTime:   recentTask.CreateTime,
			RunningTime:  util.CalcWorkflowTaskRunningTime(recentTask),
		}
	}
	if recentSucceedTask.TaskID > 0 {
		workflow.RecentSuccessfulTask = &TaskInfo{
			TaskID:       recentSucceedTask.TaskID,
			PipelineName: recentSucceedTask.WorkflowName,
			Status:       string(recentSucceedTask.Status),
			TaskCreator:  recentSucceedTask.TaskCreator,
			CreateTime:   recentSucceedTask.CreateTime,
			RunningTime:  util.CalcWorkflowTaskRunningTime(recentSucceedTask),
		}
	}
	if recentFailedTask.TaskID > 0 {
		workflow.RecentFailedTask = &TaskInfo{
			TaskID:       recentFailedTask.TaskID,
			PipelineName: recentFailedTask.WorkflowName,
			Status:       string(recentFailedTask.Status),
			TaskCreator:  recentFailedTask.TaskCreator,
			CreateTime:   recentFailedTask.CreateTime,
			RunningTime:  util.CalcWorkflowTaskRunningTime(recentFailedTask),
		}
	}
	if len(recentTenTask) > 0 {
		for _, task := range recentTenTask {
			workflow.RecentTasks = append(workflow.RecentTasks, &TaskInfo{
				TaskID:      task.TaskID,
				Status:      string(task.Status),
				TaskCreator: task.TaskCreator,
				CreateTime:  task.CreateTime,
				StartTime:   task.StartTime,
				EndTime:     task.EndTime,
				RunningTime: util.CalcWorkflowTaskRunningTime(task),
			})
		}
	}
}

func clearWorkflowV4Triggers(workflow *commonmodels.WorkflowV4) {
	workflow.HookCtls = nil
	workflow.MeegoHookCtls = nil
	workflow.GeneralHookCtls = nil
	workflow.JiraHookCtls = nil
}

func ensureWorkflowV4Resp(encryptedKey string, workflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	var buildMap sync.Map
	var buildTemplateMap sync.Map
	for _, notify := range workflow.NotifyCtls {
		err := notify.GenerateNewNotifyConfigWithOldData()
		if err != nil {
			return err
		}
	}

	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			err := ensureWorkflowV4JobResp(job, logger, &buildMap, &buildTemplateMap, encryptedKey, workflow.Project)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func ensureWorkflowV4JobResp(job *commonmodels.Job, logger *zap.SugaredLogger, buildMap *sync.Map, buildTemplateMap *sync.Map, encryptedKey, workflowProjectName string) error {
	if job.JobType == config.JobZadigBuild {
		spec := &commonmodels.ZadigBuildJobSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			logger.Errorf(err.Error())
			return e.ErrFindWorkflow.AddErr(err)
		}
		for _, build := range spec.ServiceAndBuildsOptions {
			var buildInfo *commonmodels.Build
			var err error
			buildMapValue, ok := buildMap.Load(build.BuildName)
			if !ok {
				buildInfo, err = commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: build.BuildName})
				if err != nil {
					logger.Errorf("find build: %s error: %v", build.BuildName, err)
					buildMap.Store(build.BuildName, nil)
					continue
				}
				buildMap.Store(build.BuildName, buildInfo)
			} else {
				if buildMapValue == nil {
					logger.Errorf("find build: %s error: %v", build.BuildName, err)
					continue
				}
				buildInfo = buildMapValue.(*commonmodels.Build)
			}

			kvs := buildInfo.PreBuild.Envs
			if buildInfo.TemplateID != "" {
				var templateEnvs commonmodels.KeyValList

				// if template not found, envs are empty, but do not block user.
				var buildTemplate *commonmodels.BuildTemplate
				buildTemplateMapValue, ok := buildTemplateMap.Load(buildInfo.TemplateID)
				if !ok {
					buildTemplate, err = commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{
						ID: buildInfo.TemplateID,
					})
					if err != nil {
						logger.Errorf("failed to find build template with id: %s, err: %s", buildInfo.TemplateID, err)
						buildTemplateMap.Store(buildInfo.TemplateID, nil)
					} else {
						templateEnvs = buildTemplate.PreBuild.Envs
						buildTemplateMap.Store(buildInfo.TemplateID, buildTemplate)
					}
				} else {
					if buildTemplateMapValue == nil {
						logger.Errorf("failed to find build template with id: %s, err: %s", buildInfo.TemplateID, err)
					} else {
						buildTemplate = buildTemplateMapValue.(*commonmodels.BuildTemplate)
						templateEnvs = buildTemplate.PreBuild.Envs
					}
				}

				for _, target := range buildInfo.Targets {
					if target.ServiceName == build.ServiceName && target.ServiceModule == build.ServiceModule {
						kvs = target.Envs
					}
				}

				// if build template update any keyvals, merge it.
				kvs = commonservice.MergeBuildEnvs(templateEnvs.ToRuntimeList(), kvs.ToRuntimeList()).ToKVList()
			}
			build.KeyVals = commonservice.MergeBuildEnvs(kvs.ToRuntimeList(), build.KeyVals)
			if err := commonservice.EncryptKeyVals(encryptedKey, build.KeyVals, logger); err != nil {
				logger.Errorf(err.Error())
				return e.ErrFindWorkflow.AddErr(err)
			}
		}
		job.Spec = spec
	}
	if job.JobType == config.JobZadigScanning {
		spec := &commonmodels.ZadigScanningJobSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			logger.Errorf(err.Error())
			return e.ErrFindWorkflow.AddErr(err)
		}

		scanSvc := commonservice.NewScanningService()
		for _, scanning := range spec.ScanningOptions {
			projectName := scanning.ProjectName
			if projectName == "" {
				projectName = workflowProjectName
			}
			scanningInfo, err := scanSvc.GetByName(projectName, scanning.Name)
			if err != nil {
				logger.Errorf(err.Error())
				return e.ErrFindWorkflow.AddErr(err)
			}
			scanning.KeyVals = commonservice.MergeBuildEnvs(scanningInfo.Envs.ToRuntimeList(), scanning.KeyVals)
		}
		for _, scanning := range spec.ServiceScanningOptions {
			projectName := scanning.ProjectName
			if projectName == "" {
				projectName = workflowProjectName
			}
			scanningInfo, err := scanSvc.GetByName(projectName, scanning.Name)
			if err != nil {
				logger.Errorf(err.Error())
				return e.ErrFindWorkflow.AddErr(err)
			}
			scanning.KeyVals = commonservice.MergeBuildEnvs(scanningInfo.Envs.ToRuntimeList(), scanning.KeyVals)
		}
		job.Spec = spec
	}
	if job.JobType == config.JobWorkflowTrigger {
		spec := &commonmodels.WorkflowTriggerJobSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			logger.Errorf(err.Error())
			return e.ErrFindWorkflow.AddErr(err)
		}
		for _, info := range spec.ServiceTriggerWorkflow {
			workflow, err := commonrepo.NewWorkflowV4Coll().Find(info.WorkflowName)
			if err != nil {
				logger.Errorf(err.Error())
				continue
			}
			var paramList []*commonmodels.Param
			for _, param := range workflow.Params {
				if !strings.Contains(param.Value, setting.FixedValueMark) {
					param.Source = config.ParamSourceRuntime
					paramList = append(paramList, param)
				}
			}
			info.Params = commonservice.MergeParams(paramList, info.Params)
		}
		for _, info := range spec.FixedWorkflowList {
			workflow, err := commonrepo.NewWorkflowV4Coll().Find(info.WorkflowName)
			if err != nil {
				logger.Errorf(err.Error())
				continue
			}
			var paramList []*commonmodels.Param
			for _, param := range workflow.Params {
				if !strings.Contains(param.Value, setting.FixedValueMark) {
					param.Source = config.ParamSourceRuntime
					paramList = append(paramList, param)
				}
			}
			info.Params = commonservice.MergeParams(paramList, info.Params)
		}
		job.Spec = spec
	}
	if job.JobType == config.JobFreestyle {
		spec := &commonmodels.FreestyleJobSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			logger.Errorf(err.Error())
			return e.ErrFindWorkflow.AddErr(err)
		}
		if err := commonservice.EncryptKeyVals(encryptedKey, spec.Envs, logger); err != nil {
			logger.Errorf(err.Error())
			return e.ErrFindWorkflow.AddErr(err)
		}
		job.Spec = spec
	}
	if job.JobType == config.JobPlugin {
		spec := &commonmodels.PluginJobSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			logger.Errorf(err.Error())
			return e.ErrFindWorkflow.AddErr(err)
		}
		if err := commonservice.EncryptParams(encryptedKey, spec.Plugin.Inputs, logger); err != nil {
			logger.Errorf(err.Error())
			return e.ErrFindWorkflow.AddErr(err)
		}
		job.Spec = spec
	}
	if job.JobType == config.JobZadigTesting {
		spec := &commonmodels.ZadigTestingJobSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			logger.Errorf(err.Error())
			return e.ErrFindWorkflow.AddErr(err)
		}
		job.Spec = spec
		for _, testing := range spec.ServiceTestOptions {
			testingInfo, err := commonrepo.NewTestingColl().Find(testing.Name, "")
			if err != nil {
				logger.Errorf("find testing: %s error: %s", testing.Name, err)
				continue
			}
			testing.KeyVals = commonservice.MergeBuildEnvs(testingInfo.PreTest.Envs.ToRuntimeList(), testing.KeyVals)
		}
		for _, testing := range spec.TestModuleOptions {
			testingInfo, err := commonrepo.NewTestingColl().Find(testing.Name, "")
			if err != nil {
				logger.Errorf("find testing: %s error: %s", testing.Name, err)
				continue
			}
			testing.KeyVals = commonservice.MergeBuildEnvs(testingInfo.PreTest.Envs.ToRuntimeList(), testing.KeyVals)
		}
	}

	if job.JobType == config.JobZadigVMDeploy {
		spec := &commonmodels.ZadigVMDeployJobSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			logger.Errorf(err.Error())
			return e.ErrFindWorkflow.AddErr(err)
		}
		for _, vmDeploy := range spec.ServiceAndVMDeploysOptions {
			var buildInfo *commonmodels.Build
			var err error
			buildMapValue, ok := buildMap.Load(vmDeploy.DeployName)
			if !ok {
				buildInfo, err = commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: vmDeploy.DeployName})
				if err != nil {
					logger.Errorf("find build: %s error: %v", vmDeploy.DeployName, err)
					buildMap.Store(vmDeploy.DeployName, nil)
					continue
				}
				buildMap.Store(vmDeploy.DeployName, buildInfo)
			} else {
				if buildMapValue == nil {
					logger.Errorf("find build: %s error: %v", vmDeploy.DeployName, err)
					continue
				}
				buildInfo = buildMapValue.(*commonmodels.Build)
			}

			kvs := buildInfo.PreDeploy.Envs
			// if buildInfo.TemplateID != "" {
			// 	var templateEnvs commonmodels.KeyValList

			// 	// if template not found, envs are empty, but do not block user.
			// 	var buildTemplate *commonmodels.BuildTemplate
			// 	buildTemplateMapValue, ok := buildTemplateMap.Load(buildInfo.TemplateID)
			// 	if !ok {
			// 		buildTemplate, err = commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{
			// 			ID: buildInfo.TemplateID,
			// 		})
			// 		if err != nil {
			// 			logger.Errorf("failed to find build template with id: %s, err: %s", buildInfo.TemplateID, err)
			// 			buildTemplateMap.Store(buildInfo.TemplateID, nil)
			// 		} else {
			// 			templateEnvs = buildTemplate.PreBuild.Envs
			// 			buildTemplateMap.Store(buildInfo.TemplateID, buildTemplate)
			// 		}
			// 	} else {
			// 		if buildTemplateMapValue == nil {
			// 			logger.Errorf("failed to find build template with id: %s, err: %s", buildInfo.TemplateID, err)
			// 		} else {
			// 			buildTemplate = buildTemplateMapValue.(*commonmodels.BuildTemplate)
			// 			templateEnvs = buildTemplate.PreBuild.Envs
			// 		}
			// 	}

			// 	for _, target := range buildInfo.Targets {
			// 		if target.ServiceName == vmDeploy.ServiceName && target.ServiceModule == vmDeploy.ServiceModule {
			// 			kvs = target.Envs
			// 		}
			// 	}

			// 	// if build template update any keyvals, merge it.
			// 	kvs = commonservice.MergeBuildEnvs(templateEnvs.ToRuntimeList(), kvs.ToRuntimeList()).ToKVList()
			// }
			vmDeploy.KeyVals = commonservice.MergeBuildEnvs(kvs.ToRuntimeList(), vmDeploy.KeyVals)
			if err := commonservice.EncryptKeyVals(encryptedKey, vmDeploy.KeyVals, logger); err != nil {
				logger.Errorf(err.Error())
				return e.ErrFindWorkflow.AddErr(err)
			}
		}
		job.Spec = spec
	}

	if job.JobType == config.JobNotification {
		spec := &commonmodels.NotificationJobSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			logger.Errorf(err.Error())
			return e.ErrFindWorkflow.AddErr(err)
		}

		err := spec.GenerateNewNotifyConfigWithOldData()
		if err != nil {
			logger.Errorf(err.Error())
			return e.ErrFindWorkflow.AddErr(err)
		}

		job.Spec = spec
	}
	if job.JobType == config.JobApproval {
		// to a type assertion to make sure some field goes through
		spec := &commonmodels.ApprovalJobSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			logger.Errorf(err.Error())
			return e.ErrFindWorkflow.AddErr(err)
		}

		job.Spec = spec
	}
	return nil
}

func LintWorkflowV4(workflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	workflowController := controller.CreateWorkflowController(workflow)
	err := workflowController.Validate(false)
	if err != nil {
		logger.Errorf(err.Error())
		return err
	}
	return nil
}

func createLarkApprovalDefinition(workflow *commonmodels.WorkflowV4) error {
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.Skipped {
				continue
			}

			if job.JobType == config.JobApproval {
				spec := new(commonmodels.ApprovalJobSpec)
				err := commonmodels.IToi(job.Spec, spec)
				if err != nil {
					return fmt.Errorf("failed to decode job: %s, error: %s", job.Name, err)
				}

				if data := spec.LarkApproval; data != nil && data.ID != "" {
					larkInfo, err := commonrepo.NewIMAppColl().GetByID(context.Background(), data.ID)
					if err != nil {
						return errors.Wrapf(err, "get lark app %s", data.ID)
					}
					if larkInfo.Type != string(config.LarkApproval) && larkInfo.Type != string(config.LarkApprovalIntl) {
						return errors.Errorf("lark app %s is not lark approval", data.ID)
					}

					if larkInfo.LarkApprovalCodeList == nil {
						larkInfo.LarkApprovalCodeList = make(map[string]string)
					}
					// skip if this node type approval definition already created
					if approvalNodeTypeID := larkInfo.LarkApprovalCodeList[data.GetNodeTypeKey()]; approvalNodeTypeID != "" {
						log.Infof("lark approval definition %s already created", approvalNodeTypeID)
						continue
					}

					// create this node type approval definition and save to db
					client, err := larkservice.GetLarkClientByIMAppID(data.ID)
					if err != nil {
						return errors.Wrapf(err, "get lark client by im app id %s", data.ID)
					}
					nodesArgs := make([]*lark.ApprovalNode, 0)
					for _, node := range data.ApprovalNodes {
						nodesArgs = append(nodesArgs, &lark.ApprovalNode{
							Type: node.Type,
							ApproverIDList: func() (re []string) {
								for _, user := range node.ApproveUsers {
									re = append(re, user.ID)
								}
								return
							}(),
						})
					}

					approvalCode, err := client.CreateApprovalDefinition(&lark.CreateApprovalDefinitionArgs{
						Name:        "Zadig 工作流",
						Description: "Zadig 工作流-" + data.GetNodeTypeKey(),
						Nodes:       nodesArgs,
					})
					if err != nil {
						return errors.Wrap(err, "create lark approval definition")
					}
					err = client.SubscribeApprovalDefinition(&lark.SubscribeApprovalDefinitionArgs{
						ApprovalID: approvalCode,
					})
					if err != nil {
						return errors.Wrap(err, "subscribe lark approval definition")
					}
					larkInfo.LarkApprovalCodeList[data.GetNodeTypeKey()] = approvalCode
					if err := commonrepo.NewIMAppColl().Update(context.Background(), data.ID, larkInfo); err != nil {
						return errors.Wrap(err, "update lark approval data")
					}
					log.Infof("create lark approval definition %s, key: %s", approvalCode, data.GetNodeTypeKey())
				}
			}
		}
	}
	return nil
}
func CreateGithookForWorkflowV4(ctx *internalhandler.Context, workflowName string, input *commonmodels.WorkflowV4GitHook) error {
	workflowController := controller.CreateWorkflowController(input.WorkflowArg)
	if err := workflowController.Validate(true); err != nil {
		ctx.Logger.Errorf("validate workflow error: %s", err)
		return e.ErrCreateWebhook.AddErr(err)
	}

	_, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		ctx.Logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrCreateWebhook.AddErr(err)
	}

	exists, err := commonrepo.NewWorkflowV4GitHookColl().Exists(ctx, workflowName, input.Name)
	if err != nil {
		ctx.Logger.Errorf("Failed to check if webhook %s exists: %s", input.Name, err)
		return e.ErrCreateWebhook.AddErr(err)
	}
	if exists {
		errMsg := fmt.Sprintf("webhook %s already exists", input.Name)
		ctx.Logger.Error(errMsg)
		return e.ErrCreateWebhook.AddDesc(errMsg)
	}

	if err := validateHookNames([]string{input.Name}); err != nil {
		ctx.Logger.Errorf(err.Error())
		return e.ErrCreateWebhook.AddErr(err)
	}
	err = commonservice.ProcessWebhook([]*models.WorkflowV4GitHook{input}, nil, webhook.WorkflowV4Prefix+workflowName, ctx.Logger)
	if err != nil {
		errMsg := fmt.Sprintf("failed to create webhook for workflow %s, the error is: %v", workflowName, err)
		ctx.Logger.Error(errMsg)
		return e.ErrCreateWebhook.AddDesc(errMsg)
	}

	_, err = commonrepo.NewWorkflowV4GitHookColl().Create(ctx, input)
	if err != nil {
		errMsg := fmt.Sprintf("failed to create webhook for workflow %s, the error is: %v", workflowName, err)
		ctx.Logger.Error(errMsg)
		return e.ErrCreateWebhook.AddDesc(errMsg)
	}

	if !input.IsManual {
		if err := createGerritWebhook(input.MainRepo, workflowName); err != nil {
			ctx.Logger.Errorf("create gerrit webhook failed: %v", err)
		}
	}
	return nil
}

func UpdateGithookForWorkflowV4(ctx *internalhandler.Context, workflowName string, input *commonmodels.WorkflowV4GitHook) error {
	workflowController := controller.CreateWorkflowController(input.WorkflowArg)
	if err := workflowController.Validate(true); err != nil {
		ctx.Logger.Errorf("validate workflow error: %s", err)
		return e.ErrCreateWebhook.AddErr(err)
	}

	existHook, err := commonrepo.NewWorkflowV4GitHookColl().Get(ctx, workflowName, input.Name)
	if err != nil {
		ctx.Logger.Errorf("Failed to list WorkflowV4GitHook: %s, the error is: %v", workflowName, err)
		return e.ErrUpdateWebhook.AddErr(err)
	}

	if err := validateHookNames([]string{input.Name}); err != nil {
		ctx.Logger.Errorf(err.Error())
		return e.ErrUpdateWebhook.AddErr(err)
	}
	err = commonservice.ProcessWebhook([]*models.WorkflowV4GitHook{input}, []*models.WorkflowV4GitHook{existHook}, webhook.WorkflowV4Prefix+workflowName, ctx.Logger)
	if err != nil {
		errMsg := fmt.Sprintf("failed to update webhook for workflow %s, the error is: %v", workflowName, err)
		ctx.Logger.Error(errMsg)
		return e.ErrUpdateWebhook.AddDesc(errMsg)
	}

	existHook.AutoCancel = input.AutoCancel
	existHook.Enabled = input.Enabled
	existHook.MainRepo = input.MainRepo
	existHook.Description = input.Description
	existHook.IsManual = input.IsManual
	existHook.CheckPatchSetChange = input.CheckPatchSetChange
	existHook.WorkflowArg = input.WorkflowArg

	if err := commonrepo.NewWorkflowV4GitHookColl().Update(ctx, existHook.ID.Hex(), existHook); err != nil {
		errMsg := fmt.Sprintf("failed to update webhook for workflow %s, the error is: %v", workflowName, err)
		ctx.Logger.Error(errMsg)
		return e.ErrUpdateWebhook.AddDesc(errMsg)
	}

	if !existHook.IsManual {
		if err := deleteGerritWebhook(existHook.MainRepo, workflowName); err != nil {
			ctx.Logger.Errorf("delete gerrit webhook failed: %v", err)
		}
	}
	if !input.IsManual {
		if err := createGerritWebhook(input.MainRepo, workflowName); err != nil {
			ctx.Logger.Errorf("create gerrit webhook failed: %v", err)
		}
	}
	return nil
}

func ListGithookForWorkflowV4(ctx *internalhandler.Context, workflowName string) ([]*commonmodels.WorkflowV4GitHook, error) {
	hooks, err := commonrepo.NewWorkflowV4GitHookColl().List(ctx, workflowName)
	if err != nil {
		ctx.Logger.Errorf("Failed to list WorkflowV4GitHook: %s, the error is: %v", workflowName, err)
		return []*commonmodels.WorkflowV4GitHook{}, e.ErrListWebhook.AddErr(err)
	}

	return hooks, nil
}

func GetWebhookForWorkflowV4Preset(workflowName, triggerName, ticketID string, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4GitHook, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrGetWebhook.AddErr(err)
	}
	workflowController := controller.CreateWorkflowController(workflow)
	repos, err := workflowController.GetUsedRepos()
	if err != nil {
		errMsg := fmt.Sprintf("get workflow webhook repos error: %v", err)
		log.Error(errMsg)
		return nil, e.ErrGetWebhook.AddDesc(errMsg)
	}

	var approvalTicket *commonmodels.ApprovalTicket
	if workflow.EnableApprovalTicket {
		approvalTicket, err = commonrepo.NewApprovalTicketColl().GetByID(ticketID)
		if err != nil {
			log.Errorf("cannot find approval ticket of id %s, the error is: %v", ticketID, err)
			return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
		}
	}

	if triggerName == "" {
		if err := workflowController.SetPreset(approvalTicket); err != nil {
			log.Errorf("cannot set preset for workflow %s, the error is: %v", workflowName, err)
			return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
		}

		return &commonmodels.WorkflowV4GitHook{
			Repos:       repos,
			WorkflowArg: workflowController.WorkflowV4,
		}, nil
	}

	workflowHook, err := commonrepo.NewWorkflowV4GitHookColl().Get(internalhandler.NewBackgroupContext(), workflowName, triggerName)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get webhook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return nil, e.ErrGetWebhook.AddDesc(errMsg)
	}

	workflowController = controller.CreateWorkflowController(workflowHook.WorkflowArg)
	if err := workflowController.UpdateWithLatestWorkflow(approvalTicket); err != nil {
		log.Errorf("cannot merge workflow %s's input with the latest workflow settings, the error is: %v", workflowName, err)
		return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
	}

	workflowHook.Repos = repos
	workflowHook.WorkflowArg = workflowController.WorkflowV4
	workflowHook.WorkflowArg.JiraHookCtls = nil
	workflowHook.WorkflowArg.MeegoHookCtls = nil
	workflowHook.WorkflowArg.GeneralHookCtls = nil
	workflowHook.WorkflowArg.HookCtls = nil
	return workflowHook, nil
}

func DeleteGithookForWorkflowV4(ctx *internalhandler.Context, workflowName, triggerName string) error {
	hook, err := commonrepo.NewWorkflowV4GitHookColl().Get(ctx, workflowName, triggerName)
	if err != nil {
		ctx.Logger.Errorf("Failed to get WorkflowV4GitHook: %s, error: %v", triggerName, err)
		return e.ErrDeleteWebhook.AddErr(err)
	}

	err = commonservice.ProcessWebhook(nil, []*models.WorkflowV4GitHook{hook}, webhook.WorkflowV4Prefix+workflowName, ctx.Logger)
	if err != nil {
		errMsg := fmt.Sprintf("failed to process delete webhook for workflow %s, error: %v", workflowName, err)
		ctx.Logger.Error(errMsg)
		return e.ErrDeleteWebhook.AddDesc(errMsg)
	}
	if err := commonrepo.NewWorkflowV4GitHookColl().Delete(ctx, hook.ID.Hex()); err != nil {
		errMsg := fmt.Sprintf("failed to delete webhook for workflow %s, error: %v", workflowName, err)
		ctx.Logger.Error(errMsg)
		return e.ErrDeleteWebhook.AddDesc(errMsg)
	}
	if err := deleteGerritWebhook(hook.MainRepo, workflowName); err != nil {
		ctx.Logger.Errorf("delete gerrit webhook failed: %v", err)
	}
	return nil
}

func CreateGeneralHookForWorkflowV4(workflowName string, arg *models.WorkflowV4GeneralHook, logger *zap.SugaredLogger) error {
	workflowController := controller.CreateWorkflowController(arg.WorkflowArg)
	if err := workflowController.Validate(true); err != nil {
		logger.Errorf("validate workflow error: %s", err)
		return e.ErrCreateWebhook.AddErr(err)
	}

	_, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrCreateGeneralHook.AddErr(err)
	}

	exists, err := commonrepo.NewWorkflowV4GeneralHookColl().Exists(internalhandler.NewBackgroupContext(), workflowName, arg.Name)
	if err != nil {
		logger.Errorf("Failed to check if general hook %s exists: %s", arg.Name, err)
		return e.ErrCreateGeneralHook.AddErr(err)
	}
	if exists {
		errMsg := fmt.Sprintf("general hook %s already exists", arg.Name)
		logger.Error(errMsg)
		return e.ErrCreateGeneralHook.AddDesc(errMsg)
	}

	if err = validateHookNames([]string{arg.Name}); err != nil {
		logger.Errorf(err.Error())
		return e.ErrCreateGeneralHook.AddErr(err)
	}

	_, err = commonrepo.NewWorkflowV4GeneralHookColl().Create(internalhandler.NewBackgroupContext(), arg)
	if err != nil {
		errMsg := fmt.Sprintf("failed to create general hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return e.ErrCreateGeneralHook.AddDesc(errMsg)
	}

	return nil
}

func GetGeneralHookForWorkflowV4Preset(workflowName, hookName, ticketID string, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4GeneralHook, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrGetWebhook.AddErr(err)
	}

	var approvalTicket *commonmodels.ApprovalTicket
	if workflow.EnableApprovalTicket {
		approvalTicket, err = commonrepo.NewApprovalTicketColl().GetByID(ticketID)
		if err != nil {
			log.Errorf("cannot find approval ticket of id %s, the error is: %v", ticketID, err)
			return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
		}
	}

	if hookName == "" {
		workflowController := controller.CreateWorkflowController(workflow)

		if err := workflowController.SetPreset(approvalTicket); err != nil {
			log.Errorf("cannot set preset for workflow %s, the error is: %v", workflowName, err)
			return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
		}

		return &commonmodels.WorkflowV4GeneralHook{
			WorkflowArg: workflowController.WorkflowV4,
		}, nil
	}

	workflowHook, err := commonrepo.NewWorkflowV4GeneralHookColl().Get(internalhandler.NewBackgroupContext(), workflowName, hookName)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get general hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return nil, e.ErrGetWebhook.AddDesc(errMsg)
	}

	workflowController := controller.CreateWorkflowController(workflowHook.WorkflowArg)
	if err := workflowController.UpdateWithLatestWorkflow(approvalTicket); err != nil {
		log.Errorf("cannot merge workflow %s's input with the latest workflow settings, the error is: %v", workflowName, err)
		return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
	}

	workflowHook.WorkflowArg = workflowController.WorkflowV4
	workflowHook.WorkflowArg.JiraHookCtls = nil
	workflowHook.WorkflowArg.MeegoHookCtls = nil
	workflowHook.WorkflowArg.GeneralHookCtls = nil
	workflowHook.WorkflowArg.HookCtls = nil
	return workflowHook, nil
}

func ListGeneralHookForWorkflowV4(workflowName string, logger *zap.SugaredLogger) ([]*commonmodels.WorkflowV4GeneralHook, error) {
	workflowHooks, err := commonrepo.NewWorkflowV4GeneralHookColl().List(internalhandler.NewBackgroupContext(), workflowName)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get general hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return nil, e.ErrGetWebhook.AddDesc(errMsg)
	}

	return workflowHooks, nil
}

func UpdateGeneralHookForWorkflowV4(workflowName string, arg *commonmodels.WorkflowV4GeneralHook, logger *zap.SugaredLogger) error {
	workflowController := controller.CreateWorkflowController(arg.WorkflowArg)
	if err := workflowController.Validate(true); err != nil {
		logger.Errorf("validate workflow error: %s", err)
		return e.ErrCreateWebhook.AddErr(err)
	}

	_, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrUpdateGeneralHook.AddErr(err)
	}

	workflowHook, err := commonrepo.NewWorkflowV4GeneralHookColl().Get(internalhandler.NewBackgroupContext(), workflowName, arg.Name)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get general hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return e.ErrGetWebhook.AddDesc(errMsg)
	}

	workflowHook.WorkflowArg = arg.WorkflowArg
	workflowHook.Enabled = arg.Enabled
	workflowHook.Description = arg.Description

	if err := commonrepo.NewWorkflowV4GeneralHookColl().Update(internalhandler.NewBackgroupContext(), workflowHook.ID.Hex(), workflowHook); err != nil {
		errMsg := fmt.Sprintf("failed to update general hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrUpdateGeneralHook.AddDesc(errMsg)
	}
	return nil
}

func DeleteGeneralHookForWorkflowV4(workflowName, hookName string, logger *zap.SugaredLogger) error {
	workflowHook, err := commonrepo.NewWorkflowV4GeneralHookColl().Get(internalhandler.NewBackgroupContext(), workflowName, hookName)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get general hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return e.ErrGetWebhook.AddDesc(errMsg)
	}

	if err := commonrepo.NewWorkflowV4GeneralHookColl().Delete(internalhandler.NewBackgroupContext(), workflowHook.ID.Hex()); err != nil {
		errMsg := fmt.Sprintf("failed to delete general hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return e.ErrDeleteGeneralHook.AddDesc(errMsg)
	}
	return nil
}

func GeneralHookEventHandler(workflowName, hookName string, logger *zap.SugaredLogger) error {
	_, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}

	generalHook, err := commonrepo.NewWorkflowV4GeneralHookColl().Get(internalhandler.NewBackgroupContext(), workflowName, hookName)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get general hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return e.ErrGetWebhook.AddDesc(errMsg)
	}

	if generalHook == nil {
		errMsg := fmt.Sprintf("Failed to find general hook %s", hookName)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	if !generalHook.Enabled {
		errMsg := fmt.Sprintf("Not enabled general hook %s", hookName)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	_, err = CreateWorkflowTaskV4ByBuildInTrigger(setting.GeneralHookTaskCreator, generalHook.WorkflowArg, logger)
	if err != nil {
		errMsg := fmt.Sprintf("HandleGeneralHookEvent: failed to create workflow task: %s", err)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	logger.Infof("HandleGeneralHookEvent: workflow-%s hook-%s create workflow task success", workflowName, hookName)
	return nil
}

func CreateJiraHookForWorkflowV4(workflowName string, arg *models.WorkflowV4JiraHook, logger *zap.SugaredLogger) error {
	workflowController := controller.CreateWorkflowController(arg.WorkflowArg)
	if err := workflowController.Validate(true); err != nil {
		logger.Errorf("validate workflow error: %s", err)
		return e.ErrCreateWebhook.AddErr(err)
	}

	_, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrCreateJiraHook.AddErr(err)
	}

	exists, err := commonrepo.NewWorkflowV4JiraHookColl().Exists(internalhandler.NewBackgroupContext(), workflowName, arg.Name)
	if err != nil {
		logger.Errorf("Failed to check if jira hook %s exists: %s", arg.Name, err)
		return e.ErrCreateJiraHook.AddErr(err)
	}
	if exists {
		errMsg := fmt.Sprintf("jira hook %s already exists", arg.Name)
		logger.Error(errMsg)
		return e.ErrCreateJiraHook.AddDesc(errMsg)
	}

	if err := validateHookNames([]string{arg.Name}); err != nil {
		logger.Errorf(err.Error())
		return e.ErrCreateJiraHook.AddErr(err)
	}
	if _, err = commonrepo.NewWorkflowV4JiraHookColl().Create(internalhandler.NewBackgroupContext(), arg); err != nil {
		errMsg := fmt.Sprintf("failed to create jira hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return e.ErrCreateJiraHook.AddDesc(errMsg)
	}
	return nil
}

func GetJiraHookForWorkflowV4Preset(workflowName, hookName, ticketID string, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4JiraHook, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrGetWebhook.AddErr(err)
	}

	var approvalTicket *commonmodels.ApprovalTicket
	if workflow.EnableApprovalTicket {
		approvalTicket, err = commonrepo.NewApprovalTicketColl().GetByID(ticketID)
		if err != nil {
			log.Errorf("cannot find approval ticket of id %s, the error is: %v", ticketID, err)
			return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
		}
	}

	if hookName == "" {
		workflowController := controller.CreateWorkflowController(workflow)

		if err := workflowController.SetPreset(approvalTicket); err != nil {
			log.Errorf("cannot set preset for workflow %s, the error is: %v", workflowName, err)
			return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
		}

		return &commonmodels.WorkflowV4JiraHook{
			WorkflowArg: workflowController.WorkflowV4,
		}, nil
	}

	jiraHook, err := commonrepo.NewWorkflowV4JiraHookColl().Get(internalhandler.NewBackgroupContext(), workflowName, hookName)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get jira hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return nil, e.ErrGetWebhook.AddDesc(errMsg)
	}

	workflowController := controller.CreateWorkflowController(jiraHook.WorkflowArg)
	if err := workflowController.UpdateWithLatestWorkflow(approvalTicket); err != nil {
		log.Errorf("cannot merge workflow %s's input with the latest workflow settings, the error is: %v", workflowName, err)
		return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
	}

	jiraHook.WorkflowArg = workflowController.WorkflowV4
	jiraHook.WorkflowArg.JiraHookCtls = nil
	jiraHook.WorkflowArg.MeegoHookCtls = nil
	jiraHook.WorkflowArg.GeneralHookCtls = nil
	jiraHook.WorkflowArg.HookCtls = nil
	return jiraHook, nil
}

func ListJiraHookForWorkflowV4(workflowName string, logger *zap.SugaredLogger) ([]*commonmodels.WorkflowV4JiraHook, error) {
	_, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrListJiraHook.AddErr(err)
	}

	jiraHooks, err := commonrepo.NewWorkflowV4JiraHookColl().List(internalhandler.NewBackgroupContext(), workflowName)
	if err != nil {
		logger.Errorf("Failed to list jira hooks for workflow %s, the error is: %v", workflowName, err)
		return nil, e.ErrListJiraHook.AddErr(err)
	}

	return jiraHooks, nil
}

func UpdateJiraHookForWorkflowV4(workflowName string, arg *models.WorkflowV4JiraHook, logger *zap.SugaredLogger) error {
	workflowController := controller.CreateWorkflowController(arg.WorkflowArg)
	if err := workflowController.Validate(true); err != nil {
		logger.Errorf("validate workflow error: %s", err)
		return e.ErrCreateWebhook.AddErr(err)
	}

	_, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrUpdateJiraHook.AddErr(err)
	}

	jiraHook, err := commonrepo.NewWorkflowV4JiraHookColl().Get(internalhandler.NewBackgroupContext(), workflowName, arg.Name)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get jira hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return e.ErrUpdateJiraHook.AddDesc(errMsg)
	}

	jiraHook.Enabled = arg.Enabled
	jiraHook.Description = arg.Description
	jiraHook.JiraID = arg.JiraID
	jiraHook.JiraSystemIdentity = arg.JiraSystemIdentity
	jiraHook.JiraURL = arg.JiraURL
	jiraHook.EnabledIssueStatusChange = arg.EnabledIssueStatusChange
	jiraHook.FromStatus = arg.FromStatus
	jiraHook.ToStatus = arg.ToStatus
	jiraHook.WorkflowArg = arg.WorkflowArg
	jiraHook.WorkflowArg.JiraHookCtls = nil
	jiraHook.WorkflowArg.MeegoHookCtls = nil
	jiraHook.WorkflowArg.GeneralHookCtls = nil
	jiraHook.WorkflowArg.HookCtls = nil

	if err := commonrepo.NewWorkflowV4JiraHookColl().Update(internalhandler.NewBackgroupContext(), jiraHook.ID.Hex(), jiraHook); err != nil {
		errMsg := fmt.Sprintf("failed to update jira hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrUpdateJiraHook.AddDesc(errMsg)
	}
	return nil
}

func DeleteJiraHookForWorkflowV4(workflowName, hookName string, logger *zap.SugaredLogger) error {
	jiraHook, err := commonrepo.NewWorkflowV4JiraHookColl().Get(internalhandler.NewBackgroupContext(), workflowName, hookName)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get jira hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return e.ErrDeleteJiraHook.AddDesc(errMsg)
	}

	if err := commonrepo.NewWorkflowV4JiraHookColl().Delete(internalhandler.NewBackgroupContext(), jiraHook.ID.Hex()); err != nil {
		errMsg := fmt.Sprintf("failed to delete jira hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrDeleteJiraHook.AddDesc(errMsg)
	}

	return nil
}

func CreateMeegoHookForWorkflowV4(workflowName string, arg *models.WorkflowV4MeegoHook, logger *zap.SugaredLogger) error {
	workflowController := controller.CreateWorkflowController(arg.WorkflowArg)
	if err := workflowController.Validate(true); err != nil {
		logger.Errorf("validate workflow error: %s", err)
		return e.ErrCreateWebhook.AddErr(err)
	}

	_, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrCreateMeegoHook.AddErr(err)
	}

	exists, err := commonrepo.NewWorkflowV4MeegoHookColl().Exists(internalhandler.NewBackgroupContext(), workflowName, arg.Name)
	if err != nil {
		logger.Errorf("Failed to check if meego hook %s exists: %s", arg.Name, err)
		return e.ErrCreateMeegoHook.AddErr(err)
	}
	if exists {
		errMsg := fmt.Sprintf("meego hook %s already exists", arg.Name)
		logger.Error(errMsg)
		return e.ErrCreateMeegoHook.AddDesc(errMsg)
	}

	if _, err := commonrepo.NewWorkflowV4MeegoHookColl().Create(internalhandler.NewBackgroupContext(), arg); err != nil {
		errMsg := fmt.Sprintf("failed to create meego hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return e.ErrCreateMeegoHook.AddDesc(errMsg)
	}

	return nil
}

func GetMeegoHookForWorkflowV4Preset(workflowName, hookName, ticketID string, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4MeegoHook, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrGetWebhook.AddErr(err)
	}

	var approvalTicket *commonmodels.ApprovalTicket
	if workflow.EnableApprovalTicket {
		approvalTicket, err = commonrepo.NewApprovalTicketColl().GetByID(ticketID)
		if err != nil {
			log.Errorf("cannot find approval ticket of id %s, the error is: %v", ticketID, err)
			return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
		}
	}

	if hookName == "" {
		workflowController := controller.CreateWorkflowController(workflow)

		if err := workflowController.SetPreset(approvalTicket); err != nil {
			log.Errorf("cannot set preset for workflow %s, the error is: %v", workflowName, err)
			return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
		}

		return &commonmodels.WorkflowV4MeegoHook{
			WorkflowArg: workflowController.WorkflowV4,
		}, nil
	}

	meegoHook, err := commonrepo.NewWorkflowV4MeegoHookColl().Get(internalhandler.NewBackgroupContext(), workflowName, hookName)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get meego hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return nil, e.ErrGetWebhook.AddDesc(errMsg)
	}

	workflowController := controller.CreateWorkflowController(meegoHook.WorkflowArg)
	if err := workflowController.UpdateWithLatestWorkflow(approvalTicket); err != nil {
		log.Errorf("cannot merge workflow %s's input with the latest workflow settings, the error is: %v", workflowName, err)
		return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
	}

	meegoHook.WorkflowArg = workflowController.WorkflowV4
	meegoHook.WorkflowArg.JiraHookCtls = nil
	meegoHook.WorkflowArg.MeegoHookCtls = nil
	meegoHook.WorkflowArg.GeneralHookCtls = nil
	meegoHook.WorkflowArg.HookCtls = nil
	return meegoHook, nil
}

func ListMeegoHookForWorkflowV4(workflowName string, logger *zap.SugaredLogger) ([]*commonmodels.WorkflowV4MeegoHook, error) {
	_, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrListMeegoHook.AddErr(err)
	}

	meegoHooks, err := commonrepo.NewWorkflowV4MeegoHookColl().List(internalhandler.NewBackgroupContext(), workflowName)
	if err != nil {
		logger.Errorf("Failed to list meego hooks for workflow %s, the error is: %v", workflowName, err)
		return nil, e.ErrListMeegoHook.AddErr(err)
	}

	return meegoHooks, nil
}

func UpdateMeegoHookForWorkflowV4(workflowName string, arg *models.WorkflowV4MeegoHook, logger *zap.SugaredLogger) error {
	workflowController := controller.CreateWorkflowController(arg.WorkflowArg)
	if err := workflowController.Validate(true); err != nil {
		logger.Errorf("validate workflow error: %s", err)
		return e.ErrCreateWebhook.AddErr(err)
	}

	_, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrUpdateMeegoHook.AddErr(err)
	}

	meegoHook, err := commonrepo.NewWorkflowV4MeegoHookColl().Get(internalhandler.NewBackgroupContext(), workflowName, arg.Name)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get meego hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return e.ErrUpdateMeegoHook.AddDesc(errMsg)
	}

	meegoHook.Enabled = arg.Enabled
	meegoHook.Description = arg.Description
	meegoHook.MeegoID = arg.MeegoID
	meegoHook.MeegoURL = arg.MeegoURL
	meegoHook.MeegoSystemIdentity = arg.MeegoSystemIdentity
	meegoHook.WorkflowArg = arg.WorkflowArg
	meegoHook.WorkflowArg.JiraHookCtls = nil
	meegoHook.WorkflowArg.MeegoHookCtls = nil
	meegoHook.WorkflowArg.GeneralHookCtls = nil
	meegoHook.WorkflowArg.HookCtls = nil

	if err := commonrepo.NewWorkflowV4MeegoHookColl().Update(internalhandler.NewBackgroupContext(), meegoHook.ID.Hex(), meegoHook); err != nil {
		errMsg := fmt.Sprintf("failed to update jira hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrUpdateMeegoHook.AddDesc(errMsg)
	}
	return nil
}

func DeleteMeegoHookForWorkflowV4(workflowName, hookName string, logger *zap.SugaredLogger) error {
	_, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrDeleteMeegoHook.AddErr(err)
	}

	meegoHook, err := commonrepo.NewWorkflowV4MeegoHookColl().Get(internalhandler.NewBackgroupContext(), workflowName, hookName)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get meego hook for workflow %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return e.ErrDeleteMeegoHook.AddDesc(errMsg)
	}

	if err := commonrepo.NewWorkflowV4MeegoHookColl().Delete(internalhandler.NewBackgroupContext(), meegoHook.ID.Hex()); err != nil {
		errMsg := fmt.Sprintf("failed to delete meego hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrDeleteMeegoHook.AddDesc(errMsg)
	}

	return nil
}

func BulkCopyWorkflowV4(args BulkCopyWorkflowArgs, username string, log *zap.SugaredLogger) error {
	var workflows []commonrepo.WorkflowV4

	for _, item := range args.Items {
		workflows = append(workflows, commonrepo.WorkflowV4{
			ProjectName: item.ProjectName,
			Name:        item.Old,
		})
	}
	oldWorkflows, err := commonrepo.NewWorkflowV4Coll().ListByWorkflows(commonrepo.ListWorkflowV4Opt{
		Workflows: workflows,
	})
	if err != nil {
		log.Error(err)
		return e.ErrGetPipeline.AddErr(err)
	}
	workflowMap := make(map[string]*commonmodels.WorkflowV4)
	for _, workflow := range oldWorkflows {
		workflowMap[workflow.Project+"-"+workflow.Name] = workflow
	}
	var newWorkflows []*commonmodels.WorkflowV4
	for _, workflow := range args.Items {
		if item, ok := workflowMap[workflow.ProjectName+"-"+workflow.Old]; ok {
			newItem := *item
			newItem.UpdatedBy = username
			newItem.Name = workflow.New
			newItem.DisplayName = workflow.NewDisplayName
			newItem.BaseName = workflow.BaseName
			newItem.ID = primitive.NewObjectID()
			// do not copy webhook triggers.
			newItem.HookCtls = []*commonmodels.WorkflowV4Hook{}

			newWorkflows = append(newWorkflows, &newItem)
		} else {
			return fmt.Errorf("workflow:%s not exist", item.Project+"-"+item.Name)
		}
	}
	return commonrepo.NewWorkflowV4Coll().BulkCreate(newWorkflows)
}

func CreateCronForWorkflowV4(workflowName string, input *commonmodels.Cronjob, logger *zap.SugaredLogger) error {
	workflowController := controller.CreateWorkflowController(input.WorkflowV4Args)
	if err := workflowController.Validate(true); err != nil {
		logger.Errorf("validate workflow error: %s", err)
		return e.ErrCreateWebhook.AddErr(err)
	}

	if !input.ID.IsZero() {
		return e.ErrUpsertCronjob.AddDesc("cronjob id is not empty")
	}
	input.Name = workflowName
	input.Type = setting.WorkflowV4Cronjob
	err := commonrepo.NewCronjobColl().Create(input)
	if err != nil {
		msg := fmt.Sprintf("Failed to create cron job, error: %v", err)
		log.Error(msg)
		return errors.New(msg)
	}
	if !input.Enabled {
		return nil
	}

	payload := &commonservice.CronjobPayload{
		Name:    workflowName,
		JobType: setting.WorkflowV4Cronjob,
		Action:  setting.TypeEnableCronjob,
		JobList: []*commonmodels.Schedule{cronJobToSchedule(input)},
	}

	pl, _ := json.Marshal(payload)
	err = commonrepo.NewMsgQueueCommonColl().Create(&msg_queue.MsgQueueCommon{
		Payload:   string(pl),
		QueueType: setting.TopicCronjob,
	})
	if err != nil {
		log.Errorf("Failed to publish cron to MsgQueueCommon, the error is: %v", err)
		return e.ErrUpsertCronjob.AddDesc(err.Error())
	}
	return nil
}

func UpdateCronForWorkflowV4(input *commonmodels.Cronjob, logger *zap.SugaredLogger) error {
	workflowController := controller.CreateWorkflowController(input.WorkflowV4Args)
	if err := workflowController.Validate(true); err != nil {
		logger.Errorf("validate workflow error: %s", err)
		return e.ErrCreateWebhook.AddErr(err)
	}

	_, err := commonrepo.NewCronjobColl().GetByID(input.ID)
	if err != nil {
		msg := fmt.Sprintf("cron job not exist, error: %v", err)
		log.Error(msg)
		return errors.New(msg)
	}
	if err := commonrepo.NewCronjobColl().Update(input); err != nil {
		msg := fmt.Sprintf("Failed to update cron job, error: %v", err)
		log.Error(msg)
		return errors.New(msg)
	}
	payload := &commonservice.CronjobPayload{
		Name:    input.Name,
		JobType: setting.WorkflowV4Cronjob,
		Action:  setting.TypeEnableCronjob,
	}
	if !input.Enabled {
		payload.DeleteList = []string{input.ID.Hex()}
	} else {
		payload.JobList = []*commonmodels.Schedule{cronJobToSchedule(input)}
	}

	pl, _ := json.Marshal(payload)
	err = commonrepo.NewMsgQueueCommonColl().Create(&msg_queue.MsgQueueCommon{
		Payload:   string(pl),
		QueueType: setting.TopicCronjob,
	})
	if err != nil {
		log.Errorf("Failed to publish cron to MsgQueueCommon, the error is: %v", err)
		return e.ErrUpsertCronjob.AddDesc(err.Error())
	}
	return nil
}

func ListCronForWorkflowV4(workflowName string, logger *zap.SugaredLogger) ([]*commonmodels.Cronjob, error) {
	crons, err := commonrepo.NewCronjobColl().List(&commonrepo.ListCronjobParam{
		ParentName: workflowName,
		ParentType: setting.WorkflowV4Cronjob,
	})
	if err != nil {
		logger.Errorf("Failed to list WorkflowV4 : %s cron jobs, the error is: %v", workflowName, err)
		return crons, e.ErrUpsertCronjob.AddErr(err)
	}
	return crons, nil
}

func GetCronForWorkflowV4Preset(workflowName, cronID, ticketID string, logger *zap.SugaredLogger) (*commonmodels.Cronjob, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrUpsertCronjob.AddErr(err)
	}

	var approvalTicket *commonmodels.ApprovalTicket
	if workflow.EnableApprovalTicket {
		approvalTicket, err = commonrepo.NewApprovalTicketColl().GetByID(ticketID)
		if err != nil {
			log.Errorf("cannot find approval ticket of id %s, the error is: %v", ticketID, err)
			return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
		}
	}

	if cronID == "" {
		workflowController := controller.CreateWorkflowController(workflow)
		if err := workflowController.SetPreset(approvalTicket); err != nil {
			log.Errorf("cannot set preset for workflow %s, the error is: %v", workflowName, err)
			return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
		}

		return &commonmodels.Cronjob{
			WorkflowV4Args: workflowController.WorkflowV4,
		}, nil
	}

	id, err := primitive.ObjectIDFromHex(cronID)
	if err != nil {
		logger.Errorf("Failed to parse cron id: %s, the error is: %v", cronID, err)
		return nil, e.ErrUpsertCronjob.AddErr(err)
	}
	cronJob, err := commonrepo.NewCronjobColl().GetByID(id)
	if err != nil {
		msg := fmt.Sprintf("cron job not exist, error: %v", err)
		log.Error(msg)
		return nil, errors.New(msg)
	}

	workflowController := controller.CreateWorkflowController(cronJob.WorkflowV4Args)
	if err := workflowController.UpdateWithLatestWorkflow(approvalTicket); err != nil {
		log.Errorf("cannot merge workflow %s's input with the latest workflow settings, the error is: %v", workflowName, err)
		return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
	}

	cronJob.WorkflowV4Args = workflowController.WorkflowV4
	return cronJob, nil
}

func DeleteCronForWorkflowV4(workflowName, cronID string, logger *zap.SugaredLogger) error {
	id, err := primitive.ObjectIDFromHex(cronID)
	if err != nil {
		logger.Errorf("Failed to parse cron id: %s, the error is: %v", cronID, err)
		return e.ErrUpsertCronjob.AddErr(err)
	}
	job, err := commonrepo.NewCronjobColl().GetByID(id)
	if err != nil {
		msg := fmt.Sprintf("cron job not exist, error: %v", err)
		log.Error(msg)
		return errors.New(msg)
	}
	payload := &commonservice.CronjobPayload{
		Name:       workflowName,
		JobType:    setting.WorkflowV4Cronjob,
		Action:     setting.TypeEnableCronjob,
		DeleteList: []string{job.ID.Hex()},
	}

	pl, _ := json.Marshal(payload)
	err = commonrepo.NewMsgQueueCommonColl().Create(&msg_queue.MsgQueueCommon{
		Payload:   string(pl),
		QueueType: setting.TopicCronjob,
	})
	if err != nil {
		log.Errorf("Failed to publish cron to MsgQueueCommon, the error is: %v", err)
		return e.ErrUpsertCronjob.AddDesc(err.Error())
	}
	if err := commonrepo.NewCronjobColl().Delete(&commonrepo.CronjobDeleteOption{IDList: []string{cronID}}); err != nil {
		logger.Errorf("Failed to delete cron job: %s, the error is: %v", cronID, err)
		return e.ErrUpsertCronjob.AddDesc(err.Error())
	}
	return nil
}

func cronJobToSchedule(input *commonmodels.Cronjob) *commonmodels.Schedule {
	return &commonmodels.Schedule{
		ID:             input.ID,
		Number:         input.Number,
		Frequency:      input.Frequency,
		UnixStamp:      input.UnixStamp,
		Time:           input.Time,
		MaxFailures:    input.MaxFailure,
		WorkflowV4Args: input.WorkflowV4Args,
		Type:           config.ScheduleType(input.JobType),
		Cron:           input.Cron,
		Enabled:        input.Enabled,
	}
}

func GetPatchParams(patchItem *commonmodels.PatchItem, logger *zap.SugaredLogger) ([]*commonmodels.Param, error) {
	resp := []*commonmodels.Param{}
	kvs, err := commomtemplate.GetYamlVariables(patchItem.PatchContent, logger)
	if err != nil {
		return resp, fmt.Errorf("get kv from content error: %s", err)
	}
	paramMap := map[string]*commonmodels.Param{}
	for _, param := range patchItem.Params {
		paramMap[param.Name] = param
	}
	for _, kv := range kvs {
		if param, ok := paramMap[kv.Key]; ok {
			resp = append(resp, param)
			continue
		}
		resp = append(resp, &commonmodels.Param{
			Name:       kv.Key,
			ParamsType: "string",
		})
	}
	return resp, nil
}

func GetWorkflowRepoIndex(workflow *commonmodels.WorkflowV4, currentJobName string, log *zap.SugaredLogger) []*controller.RepoIndex {
	return controller.GetWorkflowRepoIndex(workflow, currentJobName, log)
}

func GetWorkflowGlobalVars(workflow *commonmodels.WorkflowV4, currentJobName string, log *zap.SugaredLogger) ([]string, error) {
	workflowController := controller.CreateWorkflowController(workflow)
	kvs, err := workflowController.GetReferableVariables(currentJobName, controller.GetWorkflowVariablesOption{
		GetAggregatedVariables:      true,
		GetRuntimeVariables:         true,
		GetPlaceHolderVariables:     true,
		GetServiceSpecificVariables: true,
		UseUserInput:                false,
	}, true)

	if err != nil {
		log.Errorf("failed to get referable variable in workflow, error: %s", err)
		return nil, err
	}

	resp := make([]string, 0)
	for _, kv := range kvs {
		resp = append(resp, fmt.Sprintf("{{.%s}}", kv.Key))
	}

	return resp, nil
}

func getDefaultVars(workflow *commonmodels.WorkflowV4, currentJobName string) []string {
	vars := []string{}
	vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, "project"))
	vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, "workflow.name"))
	vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, "workflow.task.creator"))
	vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, "workflow.task.creator.id"))
	vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, "workflow.task.creator.userId"))
	vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, "workflow.task.timestamp"))
	vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, "workflow.task.id"))
	for _, param := range workflow.Params {
		if param.ParamsType == "repo" || param.ParamsType == "file" {
			continue
		}
		vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"workflow", "params", param.Name}, ".")))
	}
	for _, stage := range workflow.Stages {
		for _, j := range stage.Jobs {
			if j.Name == currentJobName {
				continue
			}
			switch j.JobType {
			case config.JobZadigBuild:
				spec := new(commonmodels.ZadigBuildJobSpec)
				if err := commonmodels.IToiYaml(j.Spec, spec); err != nil {
					return vars
				}
				vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"job", j.Name, "SERVICES"}, ".")))
				vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"job", j.Name, "BRANCHES"}, ".")))
				vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"job", j.Name, "IMAGES"}, ".")))
				vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"job", j.Name, "GITURLS"}, ".")))
				for _, s := range spec.ServiceAndBuilds {
					vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"job", j.Name, s.ServiceName, s.ServiceModule, "COMMITID"}, ".")))
					vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"job", j.Name, s.ServiceName, s.ServiceModule, "BRANCH"}, ".")))
					vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"job", j.Name, s.ServiceName, s.ServiceModule, "GITURL"}, ".")))
				}
			case config.JobZadigDeploy:
				vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"job", j.Name, "envName"}, ".")))
				vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"job", j.Name, "IMAGES"}, ".")))
				vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"job", j.Name, "SERVICES"}, ".")))
			case config.JobZadigDistributeImage:
				vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"job", j.Name, "IMAGES"}, ".")))
			}
		}
	}
	return vars
}

func CheckShareStorageEnabled(clusterID, jobType, identifyName, project string, logger *zap.SugaredLogger) (bool, error) {
	// if cluster id was set, we just check if the cluster has share storage enabled
	if clusterID != "" {
		return checkClusterShareStorage(clusterID)
	}
	switch jobType {
	case string(config.JobZadigBuild):
		build, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: identifyName, ProductName: project})
		if err != nil {
			return false, fmt.Errorf("find build error: %v", err)
		}
		if build.TemplateID == "" {
			clusterID = build.PreBuild.ClusterID
			break
		}
		template, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{ID: build.TemplateID})
		if err != nil {
			return false, fmt.Errorf("find build template error: %v", err)
		}
		clusterID = template.PreBuild.ClusterID
	case string(config.JobZadigTesting):
		testing, err := commonrepo.NewTestingColl().Find(identifyName, "")
		if err != nil {
			return false, fmt.Errorf("find testing error: %v", err)
		}
		clusterID = testing.PreTest.ClusterID
	case string(config.JobZadigScanning):
		scanning, err := commonrepo.NewScanningColl().Find(project, identifyName)
		if err != nil {
			return false, fmt.Errorf("find scanning error: %v", err)
		}
		clusterID = scanning.AdvancedSetting.ClusterID
	default:
		return false, fmt.Errorf("job type %s is not supported", jobType)
	}
	if clusterID == "" {
		clusterID = setting.LocalClusterID
	}
	return checkClusterShareStorage(clusterID)
}

func checkClusterShareStorage(id string) (bool, error) {
	cluster, err := commonrepo.NewK8SClusterColl().Get(id)
	if err != nil {
		return false, fmt.Errorf("find cluter error: %v", err)
	}
	if cluster.ShareStorage.NFSProperties.PVC != "" {
		return true, nil
	}
	return false, nil
}

func ListAllAvailableWorkflows(projects []string, log *zap.SugaredLogger) ([]*Workflow, error) {
	resp := make([]*Workflow, 0)
	allCustomWorkflows, err := commonrepo.NewWorkflowV4Coll().ListByProjectNames(projects)
	if err != nil {
		log.Errorf("failed to get all custom workflows, error: %s", err)
		return nil, err
	}

	for _, customWorkflow := range allCustomWorkflows {
		resp = append(resp, &Workflow{
			Name:         customWorkflow.Name,
			DisplayName:  customWorkflow.DisplayName,
			ProjectName:  customWorkflow.Project,
			UpdateTime:   customWorkflow.UpdateTime,
			CreateTime:   customWorkflow.CreateTime,
			UpdateBy:     customWorkflow.UpdatedBy,
			WorkflowType: setting.CustomWorkflowType,
			Description:  customWorkflow.Description,
			BaseName:     customWorkflow.BaseName,
		})
	}

	return resp, nil
}

func GetLatestTaskInfo(workflowInfo *Workflow) (startTime int64, creator, status string) {
	// if we found it is a custom workflow, search it in the custom workflow task
	if workflowInfo.WorkflowType == setting.CustomWorkflowType {
		taskInfo, err := getLatestWorkflowTaskV4(workflowInfo.Name)
		if err != nil {
			return 0, "", ""
		}
		return taskInfo.StartTime, taskInfo.TaskCreator, string(taskInfo.Status)
	} else {
		// otherwise it is a product workflow, WHICH IS DELETED
		return 0, "", ""
	}
}

type HelmDeployJobMergeImageResponse struct {
	Values string `json:"values"`
}

func HelmDeployJobMergeImage(ctx *internalhandler.Context, projectName, envName, serviceName, valuesYaml string, images []string, isProduction, updateServiceRevision bool) (*HelmDeployJobMergeImageResponse, error) {
	opt := &commonrepo.ProductFindOptions{Name: projectName, EnvName: envName, Production: &isProduction}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return nil, fmt.Errorf("failed to find project: %s, err: %s", projectName, err)
	}

	helmDeploySvc := helmservice.NewHelmDeployService()
	prodSvc, _, err := helmDeploySvc.GenNewEnvService(prod, serviceName, updateServiceRevision)
	if err != nil {
		return nil, err
	}

	imageMap := make(map[string]string)
	mergedContainers := []*commonmodels.Container{}
	for _, image := range images {
		imageMap[commonutil.ExtractImageName(image)] = image
	}
	for _, container := range prodSvc.Containers {
		overrideImage, ok := imageMap[container.ImageName]
		if ok {
			container.Image = overrideImage
			mergedContainers = append(mergedContainers, container)
		}
	}

	imageValuesMaps := make([]map[string]interface{}, 0)
	for _, targetContainer := range mergedContainers {
		// prepare image replace info
		replaceValuesMap, err := commonutil.AssignImageData(targetContainer.Image, commonutil.GetValidMatchData(targetContainer.ImagePath))
		if err != nil {
			return nil, fmt.Errorf("failed to pase image uri %s/%s, err %s", projectName, serviceName, err.Error())
		}
		imageValuesMaps = append(imageValuesMaps, replaceValuesMap)
	}

	// replace image into service's values.yaml
	mergedValuesYaml, err := commonutil.ReplaceImage(valuesYaml, imageValuesMaps...)
	if err != nil {
		return nil, fmt.Errorf("failed to replace image uri %s/%s, err %s", projectName, serviceName, err.Error())

	}

	return &HelmDeployJobMergeImageResponse{
		Values: mergedValuesYaml,
	}, nil
}

func GetMseOriginalServiceYaml(project, envName, serviceName, grayTag string) (string, error) {
	yamlContent, _, err := kube.FetchCurrentAppliedYaml(&kube.GeneSvcYamlOption{
		ProductName:           project,
		EnvName:               envName,
		ServiceName:           serviceName,
		UpdateServiceRevision: false,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch current applied yaml")
	}

	var yamls []string
	resources := make([]*unstructured.Unstructured, 0)
	manifests := releaseutil.SplitManifests(yamlContent)
	for _, item := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			return "", errors.Errorf("failed to decode service %s yaml to unstructured: %v", serviceName, err)
		}
		resources = append(resources, u)
	}
	deploymentNum := 0
	nameSuffix := "-mse-" + grayTag
	for _, resource := range resources {
		switch resource.GetKind() {
		case setting.Deployment:
			if deploymentNum > 0 {
				return "", errors.Errorf("service-%s: only one deployment is allowed in each service", serviceName)
			}
			deploymentNum++

			deploymentObj := &v1.Deployment{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, deploymentObj)
			if err != nil {
				return "", errors.Errorf("failed to convert service %s deployment to deployment object: %v", serviceName, err)
			}
			if deploymentObj.Spec.Selector == nil || !checkMapKeyExist(deploymentObj.Spec.Selector.MatchLabels, types.ZadigReleaseVersionLabelKey) {
				return "", errors.Errorf("service %s deployment label selector must contain %s", serviceName, types.ZadigReleaseVersionLabelKey)
			}
			if !checkMapKeyExist(deploymentObj.Spec.Template.Labels, types.ZadigReleaseVersionLabelKey) {
				return "", errors.Errorf("service %s deployment template label must contain %s", serviceName, types.ZadigReleaseVersionLabelKey)
			}
			deploymentObj.Name += nameSuffix
			deploymentObj.Spec.Replicas = pointer.Int32(1)
			deploymentObj.Labels = setMseLabels(deploymentObj.Labels, grayTag, serviceName)
			deploymentObj.Spec.Selector.MatchLabels = setMseDeploymentLabels(deploymentObj.Spec.Selector.MatchLabels, grayTag, serviceName)
			deploymentObj.Spec.Template.Labels = setMseDeploymentLabels(deploymentObj.Spec.Template.Labels, grayTag, serviceName)
			resp, err := toYaml(deploymentObj)
			if err != nil {
				return "", errors.Errorf("failed to marshal service %s deployment object: %v", serviceName, err)
			}
			yamls = append(yamls, resp)
		case setting.ConfigMap:
			cmObj := &corev1.ConfigMap{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, cmObj)
			if err != nil {
				return "", errors.Errorf("failed to convert service %s ConfigMap to object: %v", serviceName, err)
			}
			cmObj.Name += nameSuffix
			cmObj.Labels = setMseLabels(cmObj.Labels, grayTag, serviceName)
			s, err := toYaml(cmObj)
			if err != nil {
				return "", errors.Errorf("failed to marshal service %s configmap object: %v", serviceName, err)
			}
			yamls = append(yamls, s)
		case setting.Service:
			serviceObj := &corev1.Service{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, serviceObj)
			if err != nil {
				return "", errors.Errorf("failed to convert service %s Service to object: %v", serviceName, err)
			}
			serviceObj.Name += nameSuffix
			serviceObj.Labels = setMseLabels(serviceObj.Labels, grayTag, serviceName)
			serviceObj.Spec.Selector = setMseLabels(serviceObj.Spec.Selector, grayTag, serviceName)
			s, err := toYaml(serviceObj)
			if err != nil {
				return "", errors.Errorf("failed to marshal service %s service object: %v", serviceName, err)
			}
			yamls = append(yamls, s)
		case setting.Secret:
			secretObj := &corev1.Secret{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, secretObj)
			if err != nil {
				return "", errors.Errorf("failed to convert service %s Secret to object: %v", serviceName, err)
			}
			secretObj.Name += nameSuffix
			secretObj.Labels = setMseLabels(secretObj.Labels, grayTag, serviceName)
			s, err := toYaml(secretObj)
			if err != nil {
				return "", errors.Errorf("failed to marshal service %s secret object: %v", serviceName, err)
			}
			yamls = append(yamls, s)
		case setting.Ingress:
			ingressObj := &networkingv1.Ingress{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, ingressObj)
			if err != nil {
				return "", errors.Errorf("failed to convert service %s Ingress to object: %v", serviceName, err)
			}
			ingressObj.Name += nameSuffix
			ingressObj.Labels = setMseLabels(ingressObj.Labels, grayTag, serviceName)
			s, err := toYaml(ingressObj)
			if err != nil {
				return "", errors.Errorf("failed to marshal service %s ingress object: %v", serviceName, err)
			}
			yamls = append(yamls, s)
		default:
			return "", errors.Errorf("service %s resource type %s not allowed", serviceName, resource.GetKind())
		}
	}
	if deploymentNum == 0 {
		return "", errors.Errorf("service %s must contain one deployment", serviceName)
	}
	return strings.Join(yamls, "---\n"), nil
}

func RenderMseServiceYaml(productName, envName, lastGrayTag, grayTag string, service *commonmodels.MseGrayReleaseService) (string, error) {
	resources := make([]*unstructured.Unstructured, 0)
	manifests := releaseutil.SplitManifests(service.YamlContent)
	for _, item := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			return "", errors.Errorf("failed to decode service %s yaml to unstructured: %v", service.ServiceName, err)
		}
		resources = append(resources, u)
	}
	deploymentNum := 0
	var yamls []string
	serviceName := service.ServiceName
	getNameWithNewTag := func(name, lastTag, newTag string) string {
		if !strings.HasSuffix(name, lastTag) {
			return name
		}
		return strings.TrimSuffix(name, lastTag) + newTag
	}
	for _, resource := range resources {
		switch resource.GetKind() {
		case setting.Deployment:
			if deploymentNum > 0 {
				return "", errors.Errorf("service-%s: only one deployment is allowed in each service", serviceName)
			}
			deploymentNum++

			deploymentObj := &v1.Deployment{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, deploymentObj)
			if err != nil {
				return "", errors.Errorf("failed to convert service %s deployment to deployment object: %v", serviceName, err)
			}
			if deploymentObj.Spec.Selector == nil || !checkMapKeyExist(deploymentObj.Spec.Selector.MatchLabels, types.ZadigReleaseVersionLabelKey) {
				return "", errors.Errorf("service %s deployment label selector must contain %s", serviceName, types.ZadigReleaseVersionLabelKey)
			}
			if !checkMapKeyExist(deploymentObj.Spec.Template.Labels, types.ZadigReleaseVersionLabelKey) {
				return "", errors.Errorf("service %s deployment template label must contain %s", serviceName, types.ZadigReleaseVersionLabelKey)
			}

			deploymentObj.Name = getNameWithNewTag(deploymentObj.Name, lastGrayTag, grayTag)
			deploymentObj.Labels = setMseLabels(deploymentObj.Labels, grayTag, serviceName)
			deploymentObj.Spec.Selector.MatchLabels = setMseDeploymentLabels(deploymentObj.Spec.Selector.MatchLabels, grayTag, serviceName)
			deploymentObj.Spec.Template.Labels = setMseDeploymentLabels(deploymentObj.Spec.Template.Labels, grayTag, serviceName)
			Replicas := int32(service.Replicas)
			deploymentObj.Spec.Replicas = &Replicas
			resp, err := toYaml(deploymentObj)
			if err != nil {
				return "", errors.Errorf("failed to marshal service %s deployment object: %v", serviceName, err)
			}
			prod, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
				Name:    productName,
				EnvName: envName,
			})
			if err != nil {
				return "", errors.Errorf("failed to find product %s: %v", productName, err)
			}
			serviceModules := sets.NewString()
			var newImages []*commonmodels.Container
			for _, image := range service.ServiceAndImage {
				serviceModules.Insert(image.ServiceModule)
				newImages = append(newImages, &commonmodels.Container{
					Name:  image.ServiceModule,
					Image: image.Image,
				})
			}
			for _, services := range prod.Services {
				for _, productService := range services {
					for _, container := range productService.Containers {
						if !serviceModules.Has(container.Name) {
							newImages = append(newImages, &commonmodels.Container{
								Name:  container.Name,
								Image: container.Image,
							})
						}
					}
				}
			}
			resp, _, err = kube.ReplaceWorkloadImages(resp, newImages)
			if err != nil {
				return "", errors.Errorf("failed to replace service %s deployment image: %v", serviceName, err)
			}
			yamls = append(yamls, resp)
		case setting.ConfigMap:
			cmObj := &corev1.ConfigMap{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, cmObj)
			if err != nil {
				return "", errors.Errorf("failed to convert service %s ConfigMap to object: %v", serviceName, err)
			}
			cmObj.Name = getNameWithNewTag(cmObj.Name, lastGrayTag, grayTag)
			cmObj.SetLabels(setMseLabels(cmObj.GetLabels(), grayTag, serviceName))
			s, err := toYaml(cmObj)
			if err != nil {
				return "", errors.Errorf("failed to marshal service %s configmap object: %v", serviceName, err)
			}
			yamls = append(yamls, s)
		case setting.Service:
			serviceObj := &corev1.Service{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, serviceObj)
			if err != nil {
				return "", errors.Errorf("failed to convert service %s Service to object: %v", serviceName, err)
			}
			serviceObj.Name = getNameWithNewTag(serviceObj.Name, lastGrayTag, grayTag)
			serviceObj.SetLabels(setMseLabels(serviceObj.GetLabels(), grayTag, serviceName))
			serviceObj.Spec.Selector = setMseLabels(serviceObj.Spec.Selector, grayTag, serviceName)
			s, err := toYaml(serviceObj)
			if err != nil {
				return "", errors.Errorf("failed to marshal service %s service object: %v", serviceName, err)
			}
			yamls = append(yamls, s)
		case setting.Secret:
			secretObj := &corev1.Secret{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, secretObj)
			if err != nil {
				return "", errors.Errorf("failed to convert service %s Secret to object: %v", serviceName, err)
			}
			secretObj.Name = getNameWithNewTag(secretObj.Name, lastGrayTag, grayTag)
			secretObj.SetLabels(setMseLabels(secretObj.GetLabels(), grayTag, serviceName))
			s, err := toYaml(secretObj)
			if err != nil {
				return "", errors.Errorf("failed to marshal service %s secret object: %v", serviceName, err)
			}
			yamls = append(yamls, s)
		case setting.Ingress:
			ingressObj := &networkingv1.Ingress{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, ingressObj)
			if err != nil {
				return "", errors.Errorf("failed to convert service %s Ingress to object: %v", serviceName, err)
			}
			ingressObj.Name = getNameWithNewTag(ingressObj.Name, lastGrayTag, grayTag)
			ingressObj.SetLabels(setMseLabels(ingressObj.GetLabels(), grayTag, serviceName))
			s, err := toYaml(ingressObj)
			if err != nil {
				return "", errors.Errorf("failed to marshal service %s ingress object: %v", serviceName, err)
			}
			yamls = append(yamls, s)
		default:
			return "", errors.Errorf("service %s resource type %s not allowed", serviceName, resource.GetKind())
		}
	}
	if deploymentNum == 0 {
		return "", errors.Errorf("service %s must contain one deployment", serviceName)
	}
	return strings.Join(yamls, "---\n"), nil
}

func toYaml(obj runtime.Object) (string, error) {
	y := printers.YAMLPrinter{}
	writer := bytes.NewBuffer(nil)
	err := y.PrintObj(obj, writer)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal object to yaml")
	}
	return writer.String(), nil
}

func GetMseOfflineResources(grayTag, envName, projectName string) ([]string, error) {
	prod, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    projectName,
		EnvName: envName,
	})
	if err != nil {
		return nil, errors.Errorf("failed to find product %s: %v", projectName, err)
	}
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(prod.ClusterID)
	if err != nil {
		return nil, err
	}
	selector := labels.Set{
		types.ZadigReleaseTypeLabelKey:    types.ZadigReleaseTypeMseGray,
		types.ZadigReleaseVersionLabelKey: grayTag,
	}.AsSelector()
	deploymentList, err := getter.ListDeployments(prod.Namespace, selector, kubeClient)
	if err != nil {
		return nil, errors.Errorf("can't list deployment: %v", err)
	}
	var services []string
	for _, deployment := range deploymentList {
		if service := deployment.Labels[types.ZadigReleaseServiceNameLabelKey]; service != "" {
			services = append(services, service)
		} else {
			log.Warnf("GetMseOfflineResources: deployment %s has no service name label", deployment.Name)
		}
	}

	return services, nil
}

func GetBlueGreenServiceK8sServiceYaml(projectName, envName, serviceName string) (string, error) {
	yamlContent, _, err := kube.FetchCurrentAppliedYaml(&kube.GeneSvcYamlOption{
		ProductName: projectName,
		EnvName:     envName,
		ServiceName: serviceName,
	})
	if err != nil {
		return "", errors.Errorf("failed to fetch %s current applied yaml, err: %s", serviceName, err)
	}
	resources := make([]*unstructured.Unstructured, 0)
	manifests := releaseutil.SplitManifests(yamlContent)
	for _, item := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			return "", errors.Errorf("failed to decode service %s yaml to unstructured: %v", serviceName, err)
		}
		resources = append(resources, u)
	}
	var (
		service     *corev1.Service
		serviceYaml string
	)
	for _, resource := range resources {
		switch resource.GetKind() {
		case setting.Service:
			if service != nil {
				return "", errors.Errorf("service %s has more than one service", serviceName)
			}
			service = &corev1.Service{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, service)
			if err != nil {
				log.Errorf("failed to convert service %s service to service object: %v\nyaml: %s", serviceName, err, yamlContent)
				return "", errors.Errorf("failed to convert service %s service to service object: %v", serviceName, err)
			}
			service.Name = service.Name + "-blue"
			if service.Spec.Selector == nil {
				service.Spec.Selector = make(map[string]string)
			}
			service.Spec.Selector[config.BlueGreenVersionLabelName] = config.BlueVersion
			serviceYaml, err = toYaml(service)
			if err != nil {
				return "", errors.Errorf("failed to marshal service %s service object: %v", serviceName, err)
			}
		}
	}
	return serviceYaml, nil
}

func GetMseTagsInEnv(envName, projectName string) ([]string, error) {
	prod, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    projectName,
		EnvName: envName,
	})
	if err != nil {
		return nil, errors.Errorf("failed to find product %s: %v", projectName, err)
	}
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(prod.ClusterID)
	if err != nil {
		return nil, err
	}
	selector := labels.Set{
		types.ZadigReleaseTypeLabelKey: types.ZadigReleaseTypeMseGray,
	}.AsSelector()
	deploymentList, err := getter.ListDeployments(prod.Namespace, selector, kubeClient)
	if err != nil {
		return nil, errors.Errorf("can't list deployment: %v", err)
	}
	tags := sets.NewString()
	for _, deployment := range deploymentList {
		if tag := deployment.Labels[types.ZadigReleaseVersionLabelKey]; tag != "" {
			tags.Insert(tag)
		} else {
			log.Warnf("GetMseTagsInEnv: deployment %s has no release version tag", deployment.Name)
		}
	}

	return tags.List(), nil
}

func setMseDeploymentLabels(labels map[string]string, grayTag, serviceName string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[types.ZadigReleaseVersionLabelKey] = grayTag
	labels[types.ZadigReleaseMSEGrayTagLabelKey] = grayTag
	labels[types.ZadigReleaseTypeLabelKey] = types.ZadigReleaseTypeMseGray
	labels[types.ZadigReleaseServiceNameLabelKey] = serviceName
	return labels
}

func setMseLabels(labels map[string]string, grayTag, serviceName string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[types.ZadigReleaseVersionLabelKey] = grayTag
	labels[types.ZadigReleaseTypeLabelKey] = types.ZadigReleaseTypeMseGray
	labels[types.ZadigReleaseServiceNameLabelKey] = serviceName
	return labels
}

func checkMapKeyExist(m map[string]string, key string) bool {
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}

func getAllowedProjects(headers http.Header) ([]string, error) {
	type allowedProjectsData struct {
		Projects []string `json:"result"`
	}
	allowedProjects := &allowedProjectsData{}
	opaClient := opa.NewDefault()
	err := opaClient.Evaluate("rbac.user_allowed_projects", allowedProjects, func() (*opa.Input, error) {
		return generateOPAInput(headers, "GET", "/api/aslan/workflow/workflow"), nil
	})
	if err != nil {
		log.Errorf("get allowed projects error: %v", err)
		return nil, err
	}
	return allowedProjects.Projects, nil
}

func generateOPAInput(header http.Header, method string, endpoint string) *opa.Input {
	authorization := header.Get(strings.ToLower(setting.AuthorizationHeader))
	headers := map[string]string{}
	parsedPath := strings.Split(strings.Trim(endpoint, "/"), "/")
	headers[strings.ToLower(setting.AuthorizationHeader)] = authorization

	return &opa.Input{
		Attributes: &opa.Attributes{
			Request: &opa.Request{HTTP: &opa.HTTPSpec{
				Headers: headers,
				Method:  method,
			}},
		},
		ParsedPath: parsedPath,
	}
}

type JenkinsJobParams struct {
	Name    string           `json:"name"`
	Default string           `json:"default"`
	Type    config.ParamType `json:"type"`
	Choices []string         `json:"choices"`
}

func GetJenkinsJobParams(id, jobName string) ([]*JenkinsJobParams, error) {
	info, err := commonrepo.NewCICDToolColl().Get(id)
	if err != nil {
		return nil, errors.Errorf("get jenkins integration error: %v", err)
	}

	cli := jenkins.NewClient(info.URL, info.Username, info.Password)
	jobInfo, err := cli.GetJob(jobName)
	if err != nil {
		return nil, errors.Errorf("get jenkins job error: %v", err)
	}

	resp := make([]*JenkinsJobParams, 0)
	for _, definition := range jobInfo.GetParameters() {
		resp = append(resp, &JenkinsJobParams{
			Name:    definition.Name,
			Default: fmt.Sprintf("%v", definition.DefaultParameterValue.Value),
			Type: func() config.ParamType {
				if t, ok := jenkins.ParameterTypeMap[definition.Type]; ok {
					return t
				}
				return config.ParamTypeString
			}(),
			Choices: definition.Choices,
		})
	}

	return resp, nil
}

func ValidateSQL(_type config.DBInstanceType, sql string) error {
	switch _type {
	case config.DBInstanceTypeMySQL, config.DBInstanceTypeMariaDB:
		return ValidateMySQL(sql)
	default:
		return errors.Errorf("not supported db type: %s", _type)
	}
}

func ValidateMySQL(sql string) error {
	p := parser.New()

	_, _, err := p.Parse(sql, "", "")
	if err != nil {
		return errors.Errorf("parse sql statement error: %v", err)
	}
	return nil
}

func getWorkflowStatMap(workflowNames []string, workflowType config.PipelineType) map[string]*commonmodels.WorkflowStat {
	workflowStats, err := commonrepo.NewWorkflowStatColl().FindWorkflowStat(&commonrepo.WorkflowStatArgs{Names: workflowNames, Type: string(workflowType)})
	if err != nil {
		log.Warnf("Failed to list workflow stats, err: %s", err)
	}
	workflowStatMap := make(map[string]*commonmodels.WorkflowStat)
	for _, s := range workflowStats {
		workflowStatMap[s.Name] = s
	}
	return workflowStatMap
}

func setWorkflowStat(workflow *Workflow, statMap map[string]*commonmodels.WorkflowStat) {
	if s, ok := statMap[workflow.Name]; ok {
		total := float64(s.TotalSuccess + s.TotalFailure)
		successful := float64(s.TotalSuccess)
		totalDuration := float64(s.TotalDuration)

		workflow.AverageExecutionTime = totalDuration / total
		workflow.SuccessRate = successful / total
	}
}
