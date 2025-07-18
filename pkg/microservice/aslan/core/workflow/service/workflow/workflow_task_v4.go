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
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gorm.io/gorm/utils"
	"k8s.io/apimachinery/pkg/util/sets"
	controllerRuntimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	config2 "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/dingtalk"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/instantmessage"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/scmnotify"
	runtimeWorkflowController "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller"
	runtimeJobController "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller/jobcontroller"
	workwxservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workwx"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	workflowController "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	jobController "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller/job"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/apollo"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	larktool "github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/nacos"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/tool/sonar"
	workflowtool "github.com/koderover/zadig/v2/pkg/tool/workflow"
	"github.com/koderover/zadig/v2/pkg/types"
	jobspec "github.com/koderover/zadig/v2/pkg/types/job"
	"github.com/koderover/zadig/v2/pkg/types/step"
	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
)

type CreateTaskV4Resp struct {
	ProjectName  string `json:"project_name"`
	WorkflowName string `json:"workflow_name"`
	TaskID       int64  `json:"task_id"`
}

type WorkflowTaskPreview struct {
	TaskID              int64                 `bson:"task_id"                   json:"task_id"`
	WorkflowName        string                `bson:"workflow_name"             json:"workflow_key"`
	WorkflowDisplayName string                `bson:"workflow_display_name"     json:"workflow_name"`
	Params              []*commonmodels.Param `bson:"params"                    json:"params"`
	Status              config.Status         `bson:"status"                    json:"status,omitempty"`
	Reverted            bool                  `bson:"reverted"                  json:"reverted"`
	Remark              string                `bson:"remark"                    json:"remark"`
	TaskCreator         string                `bson:"task_creator"              json:"task_creator,omitempty"`
	TaskRevoker         string                `bson:"task_revoker,omitempty"    json:"task_revoker,omitempty"`
	CreateTime          int64                 `bson:"create_time"               json:"create_time,omitempty"`
	StartTime           int64                 `bson:"start_time"                json:"start_time,omitempty"`
	EndTime             int64                 `bson:"end_time"                  json:"end_time,omitempty"`
	Stages              []*StageTaskPreview   `bson:"stages"                    json:"stages"`
	ProjectName         string                `bson:"project_name"              json:"project_key"`
	Error               string                `bson:"error,omitempty"           json:"error,omitempty"`
	IsRestart           bool                  `bson:"is_restart"                json:"is_restart"`
	Debug               bool                  `bson:"debug"                     json:"debug"`
	ApprovalTicketID    string                `bson:"approval_ticket_id"        json:"approval_ticket_id"`
	ApprovalID          string                `bson:"approval_id"               json:"approval_id"`
}

type StageTaskPreview struct {
	Name       string                   `bson:"name"          json:"name"`
	Status     config.Status            `bson:"status"        json:"status"`
	StartTime  int64                    `bson:"start_time"    json:"start_time,omitempty"`
	EndTime    int64                    `bson:"end_time"      json:"end_time,omitempty"`
	Parallel   bool                     `bson:"parallel"      json:"parallel"`
	ManualExec *commonmodels.ManualExec `bson:"manual_exec"      json:"manual_exec"`
	Jobs       []*JobTaskPreview        `bson:"jobs"          json:"jobs"`
	Error      string                   `bson:"error" json:"error""`
}

type JobTaskPreview struct {
	Name                 string                       `bson:"name"           json:"name"`
	Key                  string                       `bson:"key"            json:"key"`
	DisplayName          string                       `bson:"display_name"   json:"display_name"`
	OriginName           string                       `bson:"origin_name"    json:"origin_name"`
	JobType              string                       `bson:"type"           json:"type"`
	Status               config.Status                `bson:"status"         json:"status"`
	Reverted             bool                         `bson:"reverted"       json:"reverted"`
	StartTime            int64                        `bson:"start_time"     json:"start_time,omitempty"`
	EndTime              int64                        `bson:"end_time"       json:"end_time,omitempty"`
	CostSeconds          int64                        `bson:"cost_seconds"   json:"cost_seconds"`
	Error                string                       `bson:"error"          json:"error"`
	BreakpointBefore     bool                         `bson:"breakpoint_before" json:"breakpoint_before"`
	BreakpointAfter      bool                         `bson:"breakpoint_after"  json:"breakpoint_after"`
	Spec                 interface{}                  `bson:"spec"           json:"spec"`
	ErrorPolicy          *commonmodels.JobErrorPolicy `bson:"error_policy"         yaml:"error_policy"         json:"error_policy"`
	ErrorHandlerUserID   string                       `bson:"error_handler_user_id"  yaml:"error_handler_user_id" json:"error_handler_user_id"`
	ErrorHandlerUserName string                       `bson:"error_handler_username"  yaml:"error_handler_username" json:"error_handler_username"`
	RetryCount           int                          `bson:"retry_count"           yaml:"retry_count"               json:"retry_count"`
	// JobInfo contains the fields that make up the job task name, for frontend display
	JobInfo interface{} `bson:"job_info" json:"job_info"`
}

type ZadigBuildJobSpec struct {
	Repos         []*types.Repository    `bson:"repos"           json:"repos"`
	Image         string                 `bson:"image"           json:"image"`
	Package       string                 `bson:"package"         json:"package"`
	ServiceName   string                 `bson:"service_name"    json:"service_name"`
	ServiceModule string                 `bson:"service_module"  json:"service_module"`
	Envs          []*commonmodels.KeyVal `bson:"envs"            json:"envs"`
}

type ZadigTestingJobSpec struct {
	Repos         []*types.Repository    `bson:"repos"           json:"repos"`
	JunitReport   bool                   `bson:"junit_report"    json:"junit_report"`
	Archive       bool                   `bson:"archive"         json:"archive"`
	HtmlReport    bool                   `bson:"html_report"     json:"html_report"`
	ProjectName   string                 `bson:"project_name"    json:"project_name"`
	TestName      string                 `bson:"test_name"       json:"test_name"`
	TestType      string                 `bson:"test_type"       json:"test_type"`
	ServiceName   string                 `bson:"service_name"    json:"service_name"`
	ServiceModule string                 `bson:"service_module"  json:"service_module"`
	Envs          []*commonmodels.KeyVal `bson:"envs"            json:"envs"`
}

type ZadigScanningJobSpec struct {
	TestType      string                 `bson:"scanning_type"   json:"scanning_type"`
	ServiceName   string                 `bson:"service_name"    json:"service_name"`
	ServiceModule string                 `bson:"service_module"  json:"service_module"`
	Repos         []*types.Repository    `bson:"repos"           json:"repos"`
	SonarMetrics  *step.SonarMetrics     `bson:"sonar_metrics"   json:"sonar_metrics"`
	IsHasArtifact bool                   `bson:"is_has_artifact" json:"is_has_artifact"`
	LinkURL       string                 `bson:"link_url"        json:"link_url"`
	ScanningName  string                 `bson:"scanning_name"   json:"scanning_name"`
	Envs          []*commonmodels.KeyVal `bson:"envs"            json:"envs"`
}

type ZadigDeployJobPreviewSpec struct {
	Env                string                 `bson:"env"                          json:"env"`
	EnvAlias           string                 `bson:"-"                            json:"env_alias"`
	Production         bool                   `bson:"-"                            json:"production"`
	ServiceType        string                 `bson:"service_type"                 json:"service_type"`
	DeployContents     []config.DeployContent `bson:"deploy_contents"              json:"deploy_contents"`
	SkipCheckRunStatus bool                   `bson:"skip_check_run_status"        json:"skip_check_run_status"`
	ServiceAndImages   []*ServiceAndImage     `bson:"service_and_images"           json:"service_and_images"`
	YamlContent        string                 `bson:"yaml_content"                 json:"yaml_content"`
	// UserSuppliedValue added since 1.18, the values that users gives.
	UserSuppliedValue string `bson:"user_supplied_value" json:"user_supplied_value" yaml:"user_supplied_value"`
	// VariableKVs new since 1.18, only used for k8s
	VariableKVs        []*commontypes.RenderVariableKV `bson:"variable_kvs"                 json:"variable_kvs"         yaml:"variable_kvs"`
	OriginRevision     int64                           `bson:"origin_revision"              json:"origin_revision"      yaml:"origin_revision"`
	ValueMergeStrategy config.ValueMergeStrategy       `bson:"value_merge_strategy"         json:"value_merge_strategy" yaml:"value_merge_strategy"`
}

type CustomDeployJobSpec struct {
	Image              string `bson:"image"                        json:"image"`
	Target             string `bson:"target"                       json:"target"`
	ClusterName        string `bson:"cluster_name"                 json:"cluster_name"`
	Namespace          string `bson:"namespace"                    json:"namespace"`
	SkipCheckRunStatus bool   `bson:"skip_check_run_status"        json:"skip_check_run_status"`
}

type ServiceAndImage struct {
	ServiceName   string `bson:"service_name"           json:"service_name"`
	ServiceModule string `bson:"service_module"         json:"service_module"`
	Image         string `bson:"image"                  json:"image"`
}

type K8sCanaryDeployJobSpec struct {
	Image          string               `bson:"image"                        json:"image"`
	K8sServiceName string               `bson:"k8s_service_name"             json:"k8s_service_name"`
	ClusterName    string               `bson:"cluster_name"                 json:"cluster_name"`
	Namespace      string               `bson:"namespace"                    json:"namespace"`
	ContainerName  string               `bson:"container_name"               json:"container_name"`
	CanaryReplica  int                  `bson:"canary_replica"               json:"canary_replica"`
	Events         *commonmodels.Events `bson:"events"                       json:"events"`
}

type K8sCanaryReleaseJobSpec struct {
	Image          string               `bson:"image"                        json:"image"`
	K8sServiceName string               `bson:"k8s_service_name"             json:"k8s_service_name"`
	ClusterName    string               `bson:"cluster_name"                 json:"cluster_name"`
	Namespace      string               `bson:"namespace"                    json:"namespace"`
	ContainerName  string               `bson:"container_name"               json:"container_name"`
	Events         *commonmodels.Events `bson:"events"                       json:"events"`
}

type K8sBlueGreenDeployJobSpec struct {
	Image          string               `bson:"image"                        json:"image"`
	K8sServiceName string               `bson:"k8s_service_name"             json:"k8s_service_name"`
	ClusterName    string               `bson:"cluster_name"                 json:"cluster_name"`
	Namespace      string               `bson:"namespace"                    json:"namespace"`
	ContainerName  string               `bson:"container_name"               json:"container_name"`
	Events         *commonmodels.Events `bson:"events"                       json:"events"`
}

type K8sBlueGreenReleaseJobSpec struct {
	Image          string               `bson:"image"                        json:"image"`
	K8sServiceName string               `bson:"k8s_service_name"             json:"k8s_service_name"`
	ClusterName    string               `bson:"cluster_name"                 json:"cluster_name"`
	Namespace      string               `bson:"namespace"                    json:"namespace"`
	ContainerName  string               `bson:"container_name"               json:"container_name"`
	Events         *commonmodels.Events `bson:"events"                       json:"events"`
}

type DistributeImageJobSpec struct {
	SourceRegistryID string                       `bson:"source_registry_id"           json:"source_registry_id"`
	TargetRegistryID string                       `bson:"target_registry_id"           json:"target_registry_id"`
	DistributeTarget []*step.DistributeTaskTarget `bson:"distribute_target"            json:"distribute_target"`
}

func GetWorkflowV4Preset(encryptedKey, workflowName, uid, username, ticketID string, log *zap.SugaredLogger) (*commonmodels.WorkflowV4, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		log.Errorf("cannot find workflow %s, the error is: %v", workflowName, err)
		return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
	}
	var approvalTicket *commonmodels.ApprovalTicket
	if workflow.EnableApprovalTicket {
		approvalTicket, err = commonrepo.NewApprovalTicketColl().GetByID(ticketID)
		if err != nil {
			log.Errorf("cannot find approval ticket of id %s, the error is: %v", ticketID, err)
			return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
		}
	}

	workflowCtrl := workflowController.CreateWorkflowController(workflow)

	if err := workflowCtrl.SetPreset(approvalTicket); err != nil {
		log.Errorf("failed to set preset for workflow: %s, the error is: %v", workflowName, err)
		return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
	}

	if err := ensureWorkflowV4Resp(encryptedKey, workflow, log); err != nil {
		return workflow, err
	}
	clearWorkflowV4Triggers(workflow)
	return workflow, nil
}

func GetAvailableWorkflowV4DynamicVariable(ctx *internalhandler.Context, workflow *commonmodels.WorkflowV4, jobName string) ([]string, error) {
	resp := make([]string, 0)

	workflowCtrl := workflowController.CreateWorkflowController(workflow)
	variables, err := workflowCtrl.GetReferableVariables(jobName, workflowController.GetWorkflowVariablesOption{
		GetAggregatedVariables:      false,
		GetRuntimeVariables:         false,
		GetPlaceHolderVariables:     true,
		GetServiceSpecificVariables: false,
		UseUserInput:                true,
	}, false)
	if err != nil {
		err = fmt.Errorf("failed to get render workflow variables, error: %v", err)
		ctx.Logger.Error(err)
		return nil, err
	}

	for _, kv := range variables {
		resp = append(resp, fmt.Sprintf("{{.%s}}", kv.Key))
	}

	return resp, nil
}

func GetWorkflowV4DynamicVariableValues(ctx *internalhandler.Context, workflow *commonmodels.WorkflowV4, jobName, serviceName, moduleName, key string) ([]string, error) {
	resp := make([]string, 0)

	workflowCtrl := workflowController.CreateWorkflowController(workflow)
	variables, err := workflowCtrl.GetReferableVariables(jobName, workflowController.GetWorkflowVariablesOption{
		GetAggregatedVariables:      false,
		GetRuntimeVariables:         false,
		GetPlaceHolderVariables:     false,
		GetServiceSpecificVariables: true,
		UseUserInput:                true,
	}, false)
	if err != nil {
		err = fmt.Errorf("failed to get render workflow variables, error: %v", err)
		ctx.Logger.Error(err)
		return nil, err
	}

	buildInVarMap := make(map[string]string)
	for _, kv := range variables {
		kv.Key = strings.ReplaceAll(kv.Key, "-", "_")
		kv.Key = strings.ReplaceAll(kv.Key, ".", "_")
		buildInVarMap[kv.Key] = kv.Value
	}

	resp, err = workflowCtrl.GetDynamicVariableValues(jobName, serviceName, moduleName, key, buildInVarMap)
	if err != nil {
		err = fmt.Errorf("failed to render workflow variables, error: %v", err)
		ctx.Logger.Error(err)
		return nil, err
	}

	return resp, nil
}

// CheckWorkflowV4ApprovalInitiator check if the workflow contains lark or dingtalk approval
// if so, check whether the IM information can be queried by the user's mobile phone number
func CheckWorkflowV4ApprovalInitiator(workflowName, uid string, log *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		log.Errorf("cannot find workflow %s, the error is: %v", workflowName, err)
		return e.ErrFindWorkflow.AddErr(err)
	}
	userInfo, err := user.New().GetUserByID(uid)
	if err != nil || userInfo == nil {
		return errors.New("failed to get user info by id")
	}

	// If default approval initiator is not set, check whether the user's mobile phone number can be queried
	// and only need to check once for each im app type
	isMobileChecked := map[string]bool{}
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == config.JobApproval {
				spec := new(commonmodels.ApprovalJobSpec)
				err := commonmodels.IToi(job.Spec, spec)
				if err != nil {
					return fmt.Errorf("failed to decode job: %s, error: %s", job.Name, err)
				}

				switch spec.Type {
				case config.LarkApproval:
					if spec.LarkApproval == nil {
						continue
					}
					cli, err := lark.GetLarkClientByIMAppID(spec.LarkApproval.ID)
					if err != nil {
						return errors.Errorf("failed to get lark app info by id-%s", spec.LarkApproval.ID)
					}

					if initiator := spec.LarkApproval.DefaultApprovalInitiator; initiator == nil {
						if len(userInfo.Phone) == 0 {
							return e.ErrCheckApprovalPhoneNotFound.AddDesc("phone not configured")
						}

						if isMobileChecked[spec.LarkApproval.ID] {
							continue
						}
						_, err = cli.GetUserIDByEmailOrMobile(larktool.QueryTypeMobile, userInfo.Phone, setting.LarkUserOpenID)
						if err != nil {
							return e.ErrCheckApprovalInitiator.AddDesc(fmt.Sprintf("lark app id: %s, phone: %s, error: %v",
								spec.LarkApproval.ID, userInfo.Phone, err))
						}
						isMobileChecked[spec.LarkApproval.ID] = true
					}
				case config.DingTalkApproval:
					if spec.DingTalkApproval == nil {
						continue
					}
					cli, err := dingtalk.GetDingTalkClientByIMAppID(spec.DingTalkApproval.ID)
					if err != nil {
						return errors.Errorf("failed to get dingtalk app info by id-%s", spec.DingTalkApproval.ID)
					}

					if initiator := spec.DingTalkApproval.DefaultApprovalInitiator; initiator == nil {
						if len(userInfo.Phone) == 0 {
							return e.ErrCheckApprovalPhoneNotFound.AddDesc("phone not configured")
						}

						if isMobileChecked[spec.DingTalkApproval.ID] {
							continue
						}
						_, err = cli.GetUserIDByMobile(userInfo.Phone)
						if err != nil {
							return e.ErrCheckApprovalInitiator.AddDesc(fmt.Sprintf("dingtalk app id: %s, phone: %s, error: %v",
								spec.DingTalkApproval.ID, userInfo.Phone, err))
						}
						isMobileChecked[spec.DingTalkApproval.ID] = true
					}
				case config.WorkWXApproval:
					if spec.WorkWXApproval == nil {
						continue
					}

					cli, err := workwxservice.GetLarkClientByIMAppID(spec.WorkWXApproval.ID)
					if err != nil {
						return errors.Errorf("failed to get dingtalk app info by id-%s", spec.DingTalkApproval.ID)
					}

					if initiator := spec.WorkWXApproval.CreatorUser; initiator == nil {
						if len(userInfo.Phone) == 0 {
							return e.ErrCheckApprovalPhoneNotFound.AddDesc("phone not configured")
						}

						if isMobileChecked[spec.DingTalkApproval.ID] {
							continue
						}
						phone, err := strconv.Atoi(userInfo.Phone)
						if err != nil {
							return e.ErrCheckApprovalInitiator.AddDesc("invalid phone number")
						}

						_, err = cli.FindUserByPhone(phone)
						if err != nil {
							return e.ErrCheckApprovalInitiator.AddDesc(fmt.Sprintf("workwx app id: %s, phone: %s, error: %v",
								spec.WorkWXApproval.ID, userInfo.Phone, err))
						}
						isMobileChecked[spec.DingTalkApproval.ID] = true
					}
				}
			}
		}
	}
	return nil
}

type CreateWorkflowTaskV4Args struct {
	Name               string
	Account            string
	UserID             string
	Type               config.CustomWorkflowTaskType
	ApprovalTicketID   string
	SkipWorkflowUpdate bool
}

func CreateWorkflowTaskV4ByBuildInTrigger(triggerName string, args *commonmodels.WorkflowV4, log *zap.SugaredLogger) (*CreateTaskV4Resp, error) {
	resp := &CreateTaskV4Resp{
		ProjectName:  args.Project,
		WorkflowName: args.Name,
	}

	workflowCtrl := workflowController.CreateWorkflowController(args)
	if err := workflowCtrl.UpdateWithLatestWorkflow(nil); err != nil {
		log.Errorf("cannot merge workflow %s's input with the latest workflow settings, the error is: %v", args.Name, err)
		return resp, e.ErrCreateTask.AddDesc(fmt.Sprintf("cannot merge workflow %s's input with the latest workflow settings, the error is: %v", args.Name, err))
	}

	return CreateWorkflowTaskV4(&CreateWorkflowTaskV4Args{Name: triggerName}, workflowCtrl.WorkflowV4, log)
}

func CreateWorkflowTaskV4(args *CreateWorkflowTaskV4Args, workflow *commonmodels.WorkflowV4, log *zap.SugaredLogger) (*CreateTaskV4Resp, error) {
	resp := &CreateTaskV4Resp{
		ProjectName:  workflow.Project,
		WorkflowName: workflow.Name,
	}
	if err := LintWorkflowV4(workflow, log); err != nil {
		return resp, err
	}

	var userInfo *types.UserInfo
	var err error

	workflowTask := &commonmodels.WorkflowTask{}

	// if user info exists, get user email and put it to workflow task info
	if args.UserID != "" {
		userInfo, err = user.New().GetUserByID(args.UserID)
		if err != nil || userInfo == nil {
			return resp, errors.New("failed to get user info by uid")
		}
		workflowTask.TaskCreatorEmail = userInfo.Email
		workflowTask.TaskCreatorPhone = userInfo.Phone
	}

	if args.Type == config.WorkflowTaskTypeWorkflow || args.Type == "" {
		originalWorkflow, err := commonrepo.NewWorkflowV4Coll().Find(workflow.Name)
		if err != nil {
			return resp, e.ErrCreateTask.AddErr(fmt.Errorf("cannot find workflow %s, error: %v", workflow.Name, err))
		}
		if originalWorkflow.Disabled {
			return resp, e.ErrCreateTask.AddDesc("workflow is disabled")
		}

		// do approval ticket check
		if originalWorkflow.EnableApprovalTicket {
			approvalTicket, err := commonrepo.NewApprovalTicketColl().GetByID(args.ApprovalTicketID)
			if err != nil {
				return nil, e.ErrCreateTask.AddErr(fmt.Errorf("cannot find approval ticket of id: %s, error: %s", args.ApprovalTicketID, err))
			}

			if approvalTicket.ProjectKey != originalWorkflow.Project {
				return resp, e.ErrCreateTask.AddDesc("workflow task creation denied: project key mismatch.")
			}

			// if it is not the correct time to run the workflow deny it
			if (approvalTicket.ExecutionWindowStart != 0 && time.Now().Unix() < approvalTicket.ExecutionWindowStart) ||
				(approvalTicket.ExecutionWindowEnd != 0 && time.Now().Unix() > approvalTicket.ExecutionWindowEnd) {
				return resp, e.ErrCreateTask.AddDesc("workflow task creation denied: not in execution time window.")
			}

			if len(approvalTicket.Users) != 0 {
				if args.UserID == "" {
					return resp, e.ErrCreateTask.AddDesc("workflow task creation denied: task creator cannot be identified.")
				}

				found := false
				for _, allowedUser := range approvalTicket.Users {
					if allowedUser.Email == userInfo.Email {
						found = true
						break
					}
				}

				if !found {
					return resp, e.ErrCreateTask.AddDesc(fmt.Sprintf("workflow task creation denied: user %s is not allowed to create workflow task.", userInfo.Name))
				}
			}

			workflowTask.ApprovalTicketID = args.ApprovalTicketID
			workflowTask.ApprovalID = approvalTicket.ApprovalID
		}

		// Always use the latest workflow's notification settings
		workflow.NotifyCtls = originalWorkflow.NotifyCtls
		workflowTask.Hash = originalWorkflow.Hash
	} else {
		if workflow.Disabled {
			return resp, e.ErrCreateTask.AddDesc("workflow is disabled")
		}
	}

	// if account is not set, use name as account
	if args.Account == "" {
		args.Account = args.Name
	}

	// save workflow original workflow task args.
	originTaskArgs := &commonmodels.WorkflowV4{}
	if err := commonmodels.IToi(workflow, originTaskArgs); err != nil {
		log.Errorf("save original workflow args error: %v", err)
		return resp, e.ErrCreateTask.AddDesc(err.Error())
	}

	workflowInputArgsController := workflowController.CreateWorkflowController(originTaskArgs)
	err = workflowInputArgsController.ClearOptions()
	if err != nil {
		log.Errorf("failed to remove the job options in the workflow parameters, error: %s", err)
		return resp, fmt.Errorf("failed to remove the job options in the workflow parameters, error: %s", err)
	}
	originTaskArgs.HookCtls = nil
	originTaskArgs.MeegoHookCtls = nil
	originTaskArgs.JiraHookCtls = nil
	originTaskArgs.GeneralHookCtls = nil
	workflowTask.OriginWorkflowArgs = workflowInputArgsController.WorkflowV4
	nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.WorkflowTaskV4Fmt, workflow.Name))
	if err != nil {
		log.Errorf("Counter.GetNextSeq error: %v", err)
		return resp, e.ErrGetCounter.AddDesc(err.Error())
	}
	resp.TaskID = nextTaskID

	workflowTask.TaskID = nextTaskID
	workflowTask.TaskCreator = args.Name
	workflowTask.TaskCreatorAccount = args.Account
	workflowTask.TaskCreatorID = args.UserID
	workflowTask.TaskRevoker = args.Name
	workflowTask.TaskRevokerID = args.UserID
	workflowTask.CreateTime = time.Now().Unix()
	workflowTask.WorkflowName = workflow.Name
	workflowTask.WorkflowDisplayName = workflow.DisplayName
	workflowTask.ProjectName = workflow.Project
	workflowTask.Params = workflow.Params
	workflowTask.ShareStorages = workflow.ShareStorages
	workflowTask.IsDebug = workflow.Debug
	workflowTask.Remark = workflow.Remark

	workflowCtrl := workflowController.CreateWorkflowController(workflow)
	if (args.Type == config.WorkflowTaskTypeWorkflow || args.Type == "") && !args.SkipWorkflowUpdate {
		err = workflowCtrl.UpdateWithLatestWorkflow(nil)
		if err != nil {
			log.Errorf("failed to update workflow task args with latest workflow settings, error: %s", err)
			return nil, e.ErrCreateTask.AddErr(err)
		}

		err = workflowCtrl.Validate(true)
		if err != nil {
			log.Errorf("failed to validate workflow task args, error: %s", err)
			return nil, e.ErrCreateTask.AddErr(err)
		}
	}

	workflowCtrl.SetParameterRepoCommitInfo()
	stageTasks, err := workflowCtrl.ToJobTasks(nextTaskID, args.Name, args.Account, args.UserID)
	if err != nil {
		log.Errorf("failed to generate workflow tasks from input, error: %s", err)
		return nil, e.ErrCreateTask.AddErr(err)
	}

	workflowTask.Stages = stageTasks

	if err := workflowTaskLint(workflowTask, log); err != nil {
		return resp, err
	}

	if err := createLarkApprovalDefinition(workflow); err != nil {
		return resp, errors.Wrap(err, "create lark approval definition")
	}

	workflow.HookCtls = nil
	workflow.JiraHookCtls = nil
	workflow.MeegoHookCtls = nil
	workflow.GeneralHookCtls = nil
	workflowTask.WorkflowArgs = workflow
	workflowTask.Status = config.StatusCreated
	workflowTask.StartTime = time.Now().Unix()
	workflowTask.Type = args.Type
	if args.Type == "" {
		workflowTask.Type = config.WorkflowTaskTypeWorkflow
	}

	workflowTask.WorkflowArgs, _, err = service.FillServiceModules2Jobs(workflowTask.WorkflowArgs)
	if err != nil {
		log.Errorf("fill serviceModules to jobs error: %v", err)
		return resp, e.ErrCreateTask.AddDesc(err.Error())
	}

	if err := instantmessage.NewWeChatClient().SendWorkflowTaskNotifications(workflowTask); err != nil {
		log.Errorf("send workflow task notification failed, error: %v", err)
	}

	if err := runtimeWorkflowController.CreateTask(workflowTask); err != nil {
		log.Errorf("create workflow task error: %v", err)
		return resp, e.ErrCreateTask.AddDesc(err.Error())
	}
	// Updating the comment in the git repository, this will not cause the function to return error if this function call fails
	if err := scmnotify.NewService().UpdateWebhookCommentForWorkflowV4(workflowTask, log); err != nil {
		log.Warnf("Failed to update comment for custom workflow %s, taskID: %d the error is: %s", workflowTask.WorkflowName, workflowTask.TaskID, err)
	}
	if err := scmnotify.NewService().UpdateGitCheckForWorkflowV4(workflowTask.WorkflowArgs, workflowTask.TaskID, log); err != nil {
		log.Warnf("Failed to update github check status for custom workflow %s, taskID: %d the error is: %s", workflowTask.WorkflowName, workflowTask.TaskID, err)
	}

	return resp, nil
}

func GetManualExecWorkflowTaskV4Info(workflowName string, taskID int64, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4, error) {
	originWorkflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		log.Errorf("find workflowV4 error: %s", err)
		return nil, e.ErrFindWorkflow.AddErr(err)
	}

	task, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		logger.Errorf("find workflowTaskV4 error: %s", err)
		return nil, e.ErrGetTask.AddErr(err)
	}

	var approvalTicket *commonmodels.ApprovalTicket
	if originWorkflow.EnableApprovalTicket {
		approvalTicket, err = commonrepo.NewApprovalTicketColl().GetByID(task.ApprovalTicketID)
		if err != nil {
			log.Errorf("cannot find approval ticket of id %s, the error is: %v", task.ApprovalTicketID, err)
			return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
		}
	}

	workflowCtrl := workflowController.CreateWorkflowController(task.OriginWorkflowArgs)
	err = workflowCtrl.UpdateWithLatestWorkflow(approvalTicket)
	if err != nil {
		log.Errorf("failed to set preset for workflow: %s, the error is: %v", workflowName, err)
		return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
	}
	return workflowCtrl.WorkflowV4, nil
}

func CloneWorkflowTaskV4(workflowName string, taskID int64, isView bool, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4, error) {
	originalWorkflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("find workflowV4 error: %s", err)
		return nil, e.ErrFindWorkflow.AddErr(err)
	}

	if originalWorkflow.EnableApprovalTicket && !isView {
		return nil, e.ErrCloneTask.AddDesc("无法克隆开启了预审批的工作流")
	}

	task, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		logger.Errorf("find workflowTaskV4 error: %s", err)
		return nil, e.ErrGetTask.AddErr(err)
	}

	workflowCtrl := workflowController.CreateWorkflowController(task.OriginWorkflowArgs)
	err = workflowCtrl.UpdateWithLatestWorkflow(nil)
	if err != nil {
		log.Errorf("failed to set preset for workflow: %s, the error is: %v", workflowName, err)
		return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
	}

	task.OriginWorkflowArgs.NotifyCtls = originalWorkflow.NotifyCtls
	return task.OriginWorkflowArgs, nil
}

func RetryWorkflowTaskV4(workflowName string, taskID int64, logger *zap.SugaredLogger) error {
	task, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		logger.Errorf("find workflowTaskV4 error: %s", err)
		return e.ErrGetTask.AddErr(err)
	}
	switch task.Status {
	case config.StatusFailed, config.StatusTimeout, config.StatusCancelled, config.StatusReject:
	default:
		return errors.New("工作流任务状态无法重试")
	}

	if task.OriginWorkflowArgs == nil || task.OriginWorkflowArgs.Stages == nil {
		return errors.New("工作流任务数据异常, 无法重试")
	}

	globalKeyMap := make(map[string]string)
	jobTaskMap := make(map[string]*commonmodels.JobTask)
	for _, stage := range task.WorkflowArgs.Stages {
		for _, job := range stage.Jobs {
			if job.Skipped {
				continue
			}
			ctrl, err := jobController.CreateJobController(job, task.WorkflowArgs)
			if err != nil {
				return errors.Errorf("init job controller %s error: %s", job.Name, err)
			}
			kvs, err := ctrl.GetVariableList(job.Name,
				false,
				false,
				false,
				true,
				true,
			)
			if err != nil {
				return err
			}

			for _, kv := range kvs {
				if kv.GetValue() != "" && !strings.HasPrefix(kv.GetValue(), "{{.") {
					globalKeyMap[kv.Key] = kv.GetValue()
					log.Infof("insert key %s with value %s", kv.Key, kv.GetValue())
				} else {
					log.Warnf("key %s skipped due to no value or reference value: %s", kv.Key, kv.GetValue())
				}
			}

			jobTasks, err := ctrl.ToTask(taskID)
			if err != nil {
				return errors.Errorf("job %s toJobs error: %s", job.Name, err)
			}
			for _, jobTask := range jobTasks {
				jobTaskMap[jobTask.Name] = jobTask
			}
		}
	}

	for _, stage := range task.Stages {
		if stage.Status == config.StatusPassed || stage.Status == config.StatusSkipped {
			continue
		}
		stage.Status = ""
		stage.StartTime = 0
		stage.EndTime = 0
		stage.Error = ""

		for _, jobTask := range stage.Jobs {
			if jobTask.Status == config.StatusPassed {
				continue
			}
			jobTask.Status = ""
			jobTask.StartTime = 0
			jobTask.EndTime = 0
			jobTask.Error = ""
			if t, ok := jobTaskMap[jobTask.Name]; ok {
				taskBytes, _ := json.Marshal(t)
				taskString := string(taskBytes)
				for k, v := range globalKeyMap {
					taskString = strings.ReplaceAll(taskString, fmt.Sprintf("{{.%s}}", k), v)
					log.Infof("replacing key %s with value: %s", fmt.Sprintf("{{.%s}}", k), v)
				}
				err := json.Unmarshal([]byte(taskString), &t)
				if err != nil {
					return fmt.Errorf("failed to replace input variable for task: %s, error: %s", t.Name, err)
				}
				jobTask.Spec = t.Spec
			} else {
				return errors.Errorf("failed to get jobTask %s origin spec", jobTask.Name)
			}
		}
	}

	task.Status = config.StatusCreated
	task.StartTime = time.Now().Unix()
	if err := instantmessage.NewWeChatClient().SendWorkflowTaskNotifications(task); err != nil {
		log.Errorf("send workflow task notification failed, error: %v", err)
	}

	if err := runtimeWorkflowController.UpdateTask(task); err != nil {
		log.Errorf("retry workflow task error: %v", err)
		return e.ErrCreateTask.AddDesc(fmt.Sprintf("重试工作流任务失败: %s", err.Error()))
	}

	return nil
}

type ManualExecWorkflowTaskV4Request struct {
	Jobs []*commonmodels.Job `json:"jobs"`
}

func ManualExecWorkflowTaskV4(workflowName string, taskID int64, stageName string, jobs []*commonmodels.Job, executorID, executorAccount, executorName string, isSystemAdmin bool, logger *zap.SugaredLogger) error {
	task, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		logger.Errorf("find workflowTaskV4 error: %s", err)
		return e.ErrGetTask.AddErr(err)
	}
	switch task.Status {
	case config.StatusPause:
	default:
		return errors.New("工作流任务状态无法手动执行")
	}

	if task.OriginWorkflowArgs == nil || task.OriginWorkflowArgs.Stages == nil {
		return errors.New("工作流任务数据异常, 无法手动执行")
	}

	originJobs := []*commonmodels.Job{}
	if err := commonmodels.IToi(&jobs, &originJobs); err != nil {
		log.Errorf("save original jobs error: %v", err)
		return e.ErrCreateTask.AddErr(fmt.Errorf("save original jobs error: %v", err))
	}

	globalKeyMap := make(map[string]string)

	for _, stage := range task.WorkflowArgs.Stages {
		if stage.Name == stageName {
			for _, job := range stage.Jobs {
				ctrl, err := jobController.CreateJobController(job, task.WorkflowArgs)
				if err != nil {
					log.Errorf("failed to remove the job options in the workflow parameters for job %s, stage: %s, error: %s", job.Name, stage.Name, err)
					return fmt.Errorf("failed to remove the job options in the workflow parameters for job %s, stage: %s, error: %s", job.Name, stage.Name, err)
				}

				ctrl.ClearOptions()

				err = ctrl.SetRepoCommitInfo()
				if err != nil {
					log.Errorf("failed to set repo commit info for job: %s in workflow:%s, error: %v", job.Name, task.WorkflowArgs.Name, err)
					return e.ErrCreateTask.AddDesc(err.Error())
				}

				job.Spec = ctrl.GetSpec()
			}

			stage.Jobs = jobs
		}
	}

	for _, stage := range task.OriginWorkflowArgs.Stages {
		for _, job := range stage.Jobs {
			ctrl, err := jobController.CreateJobController(job, task.WorkflowArgs)
			if err != nil {
				log.Errorf("failed to remove the job options in the workflow parameters for job %s, stage: %s, error: %s", job.Name, stage.Name, err)
				return fmt.Errorf("failed to remove the job options in the workflow parameters for job %s, stage: %s, error: %s", job.Name, stage.Name, err)
			}

			err = ctrl.SetRepoCommitInfo()
			if err != nil {
				log.Errorf("failed to set repo commit info for job: %s in workflow:%s, error: %v", job.Name, task.WorkflowArgs.Name, err)
				return e.ErrCreateTask.AddDesc(err.Error())
			}

			kvs, err := ctrl.GetVariableList(job.Name,
				false,
				false,
				false,
				true,
				true,
			)
			if err != nil {
				return err
			}

			for _, kv := range kvs {
				if kv.GetValue() != "" && !strings.HasPrefix(kv.GetValue(), "{{.") {
					globalKeyMap[kv.Key] = kv.GetValue()
					log.Infof("insert key %s with value %s", kv.Key, kv.GetValue())
				} else {
					log.Warnf("key %s skipped due to no value or reference value: %s", kv.Key, kv.GetValue())
				}
			}
		}
	}

	for _, stage := range task.OriginWorkflowArgs.Stages {
		if stage.Name == stageName {
			stage.Jobs = originJobs
		}
	}
	for _, stage := range task.WorkflowArgs.Stages {
		if stage.Name == stageName {
			stage.Jobs = originJobs
		}
	}

	workflowCtrl := workflowController.CreateWorkflowController(task.WorkflowArgs)
	if err := workflowCtrl.RenderWorkflowDefaultParams(task.TaskID, task.TaskCreator, task.TaskCreatorAccount, task.TaskCreatorID); err != nil {
		log.Errorf("RenderGlobalVariables error: %v", err)
		return e.ErrCreateTask.AddDesc(err.Error())
	}

	// set workflow params repo info, like commitid, branch etc.
	workflowCtrl.SetParameterRepoCommitInfo()

	foundStage := false
	newStageNameJobTasksMap := make(map[string][]*commonmodels.JobTask, 0)
	for _, stage := range task.WorkflowArgs.Stages {
		if stage.Name == stageName {
			foundStage = true
		}

		// is target stage and follow-up stages
		if foundStage {
			newStageNameJobTasksMap[stage.Name] = make([]*commonmodels.JobTask, 0)

			for _, job := range stage.Jobs {
				if job.Skipped {
					continue
				}

				ctrl, err := jobController.CreateJobController(job, task.WorkflowArgs)
				if err != nil {
					return errors.Errorf("init job controller %s error: %s", job.Name, err)
				}

				jobTasks, err := ctrl.ToTask(taskID)
				if err != nil {
					return errors.Errorf("job %s toJobs error: %s", job.Name, err)
				}

				job.Spec = ctrl.GetSpec()
				for _, task := range jobTasks {
					taskBytes, _ := json.Marshal(task)
					taskString := string(taskBytes)
					for k, v := range globalKeyMap {
						taskString = strings.ReplaceAll(taskString, fmt.Sprintf("{{.%s}}", k), v)
						log.Infof("replacing key %s with value: %s", fmt.Sprintf("{{.%s}}", k), v)
					}
					err := json.Unmarshal([]byte(taskString), &task)
					if err != nil {
						return fmt.Errorf("failed to replace input variable for task: %s, error: %s", task.Name, err)
					}
				}

				for _, jobTask := range jobTasks {
					jobTask.Status = ""
					jobTask.StartTime = 0
					jobTask.EndTime = 0
					jobTask.Error = ""

					if job.RunPolicy == config.SkipRun {
						jobTask.Status = config.StatusSkipped
					}
					newStageNameJobTasksMap[stage.Name] = append(newStageNameJobTasksMap[stage.Name], jobTask)
				}
			}
		}
	}

	found := false
	var preStage *commonmodels.StageTask
	for _, stage := range task.Stages {
		if stage.Status == config.StatusPassed || stage.Status == config.StatusSkipped {
			preStage = stage
			continue
		}

		// for the manual executed stage itself, we need to re-render the tasks, not getting them from the previous task since it might not be right
		if stage.Name == stageName {
			if preStage != nil && !(preStage.Status == config.StatusPassed || preStage.Status == config.StatusSkipped) {
				return errors.Errorf("previous stage %s status is not passed or skipped", preStage.Name)
			}

			if stage.ManualExec == nil || !stage.ManualExec.Enabled {
				return errors.Errorf("stage %s is not enabled for manual execution", stage.Name)
			}
			stage.ManualExec.Excuted = true

			approval := false
			if isSystemAdmin {
				approval = true
			}

			for _, user := range stage.ManualExec.ManualExecUsers {
				if user.Type == setting.UserTypeTaskCreator {
					if executorID == task.TaskCreatorID {
						approval = true
						break
					}
				}
			}
			if !approval {
				users, _ := util.GeneFlatUsers(stage.ManualExec.ManualExecUsers)
				for _, user := range users {
					if user.UserID == executorID {
						approval = true
						break
					}
				}
			}

			if !approval {
				return errors.Errorf("user %s is not allowed to manually execute stage %s", executorID, stage.Name)
			}

			found = true
			stage.ManualExec.ManualExectorID = executorID
			stage.ManualExec.ManualExectorName = executorName
		}

		if newStageNameJobTasksMap[stage.Name] != nil {
			stage.Status = ""
			stage.StartTime = 0
			stage.EndTime = 0
			stage.Error = ""
			stage.Jobs = newStageNameJobTasksMap[stage.Name]
		}

		preStage = stage
	}

	if !found {
		return errors.Errorf("stage %s not found in workflow %s or status is passed", stageName, workflowName)
	}

	if err := workflowTaskLint(task, logger); err != nil {
		return err
	}

	task.WorkflowArgs, _, err = service.FillServiceModules2Jobs(task.WorkflowArgs)
	if err != nil {
		log.Errorf("fill serviceModules to jobs error: %v", err)
		return e.ErrCreateTask.AddDesc(err.Error())
	}

	task.Status = config.StatusCreated
	if err := instantmessage.NewWeChatClient().SendWorkflowTaskNotifications(task); err != nil {
		log.Errorf("send workflow task notification failed, error: %v", err)
	}

	if err := runtimeWorkflowController.UpdateTask(task); err != nil {
		log.Errorf("manual execute workflow task error: %v", err)
		return e.ErrCreateTask.AddDesc(fmt.Sprintf("手动执行工作流任务失败: %s", err.Error()))
	}

	return nil
}

func SetWorkflowTaskV4Breakpoint(workflowName, jobName string, taskID int64, set bool, position string, logger *zap.SugaredLogger) error {
	event := &runtimeWorkflowController.WorkflowDebugEvent{
		EventType: runtimeWorkflowController.WorkflowDebugEventSetBreakPoint,
		JobName:   jobName,
		TaskID:    taskID,
		Set:       set,
		Position:  position,
	}
	bytes, _ := json.Marshal(event)
	err := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Publish(runtimeWorkflowController.WorkflowDebugChanKey(workflowName, taskID), string(bytes))
	if err != nil {
		return e.ErrStopDebugShell.AddDesc(fmt.Sprintf("failed to set workflow breakpoint, err: %s", err))
	}
	return nil
}

func EnableDebugWorkflowTaskV4(workflowName string, taskID int64, logger *zap.SugaredLogger) error {
	event := &runtimeWorkflowController.WorkflowDebugEvent{
		EventType: runtimeWorkflowController.WorkflowDebugEventSetBreakPoint,
		TaskID:    taskID,
	}
	bytes, _ := json.Marshal(event)
	err := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Publish(runtimeWorkflowController.WorkflowDebugChanKey(workflowName, taskID), string(bytes))
	if err != nil {
		return e.ErrEnableDebug.AddDesc(fmt.Sprintf("failed to set workflow breakpoint, err: %s", err))
	}
	return nil
}

func StopDebugWorkflowTaskJobV4(workflowName, jobName string, taskID int64, position string, logger *zap.SugaredLogger) error {
	event := &runtimeWorkflowController.WorkflowDebugEvent{
		EventType: runtimeWorkflowController.WorkflowDebugEventDeleteDebug,
		JobName:   jobName,
		Position:  position,
		TaskID:    taskID,
	}
	bytes, _ := json.Marshal(event)
	err := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Publish(runtimeWorkflowController.WorkflowDebugChanKey(workflowName, taskID), string(bytes))
	if err != nil {
		return e.ErrEnableDebug.AddDesc(fmt.Sprintf("failed to set workflow breakpoint, err: %s", err))
	}
	return nil
}

type CommonRevertInput struct {
	Detail string `json:"detail"`
}

type SQLRevertInput struct {
	CommonRevertInput `json:",inline"`
	SQL               string `json:"sql"`
}

type NacosRevertInput struct {
	CommonRevertInput `json:",inline"`
	NacosDatas        []*commonmodels.NacosData `json:"nacos_datas"`
}

type ApolloRevertInput struct {
	CommonRevertInput `json:",inline"`
	ApolloDatas       []*commonmodels.ApolloNamespace `json:"apollo_datas"`
}

func RevertWorkflowTaskV4Job(ctx *internalhandler.Context, workflowName, jobName string, taskID int64, input interface{}, userName, userID string, logger *zap.SugaredLogger) error {
	task, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		logger.Errorf("find workflowTaskV4 error: %s", err)
		return e.ErrGetTask.AddErr(err)
	}

	for _, stage := range task.Stages {
		for _, job := range stage.Jobs {
			if job.Name == jobName {
				switch job.JobType {
				case string(config.JobZadigDeploy):
					err = commonutil.CheckZadigProfessionalLicense()
					if err != nil {
						return err
					}

					inputSpec := new(CommonRevertInput)
					err = commonmodels.IToi(input, inputSpec)
					if err != nil {
						return fmt.Errorf("failed to decode deploy revert job spec, error: %s", err)
					}

					jobTaskSpec := &commonmodels.JobTaskDeploySpec{}
					if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
						logger.Error(err)
						return fmt.Errorf("failed to decode nacos job spec, error: %s", err)
					}

					job.Reverted = true
					task.Reverted = true
					err = commonrepo.NewworkflowTaskv4Coll().Update(task.ID.Hex(), task)
					if err != nil {
						err = fmt.Errorf("failed to update nacos job revert information, error: %s", err)
						log.Error(err)
						return err
					}

					envSvcVersionYaml, err := commonservice.GetEnvServiceVersionYaml(ctx, task.ProjectName, jobTaskSpec.Env, jobTaskSpec.ServiceName, jobTaskSpec.OriginRevision, false, jobTaskSpec.Production, logger)
					if err != nil {
						err = fmt.Errorf("failed to get env service version yaml, error: %s", err)
						log.Error(err)
						return err
					}
					revertSpec := &commonmodels.JobTaskDeployRevertSpec{
						Env:                jobTaskSpec.Env,
						ServiceName:        jobTaskSpec.ServiceName,
						ServiceType:        jobTaskSpec.ServiceType,
						Production:         jobTaskSpec.Production,
						Containers:         envSvcVersionYaml.Containers,
						Yaml:               envSvcVersionYaml.Yaml,
						VariableYaml:       envSvcVersionYaml.VariableYaml,
						OverrideKVs:        envSvcVersionYaml.OverrideKVs,
						Revision:           jobTaskSpec.OriginRevision,
						RevisionCreateTime: envSvcVersionYaml.CreateTime,
						JobTaskCommonRevertSpec: commonmodels.JobTaskCommonRevertSpec{
							Detail: inputSpec.Detail,
						},
					}

					rollbackStatus, err := commonservice.RollbackEnvServiceVersion(ctx, task.ProjectName, jobTaskSpec.Env, jobTaskSpec.ServiceName, jobTaskSpec.OriginRevision, false, jobTaskSpec.Production, inputSpec.Detail, logger)
					if err != nil {
						log.Errorf("failed to rollback env service version, error: %s", err)
						return err
					}

					revert := &commonmodels.WorkflowTaskRevert{
						TaskID:        taskID,
						WorkflowName:  workflowName,
						JobName:       jobName,
						RevertSpec:    revertSpec,
						CreateTime:    time.Now().Unix(),
						TaskCreator:   userName,
						TaskCreatorID: userID,
						Status:        config.StatusRunning,
					}
					revertID, err := commonrepo.NewWorkflowTaskRevertColl().Create(revert)
					if err != nil {
						log.Warnf("failed to insert revert task logs, error: %s", err)
					}

					if jobTaskSpec.Timeout == 0 {
						jobTaskSpec.Timeout = setting.DeployTimeout
					}

					go func() {
						var (
							err        error
							env        *commonmodels.Product
							kubeClient controllerRuntimeClient.Client
							status     = config.StatusFailed
						)
						defer func() {
							err = commonrepo.NewWorkflowTaskRevertColl().UpateStatusByID(revertID, status)
							if err != nil {
								log.Errorf("failed to update revert task status, error: %s", err)
							}
						}()

						if jobTaskSpec.ServiceType == setting.K8SDeployType {
							jobTaskSpec.RelatedPodLabels = rollbackStatus.RelatedPodLabels
							jobTaskSpec.ReplaceResources = rollbackStatus.ReplaceResources

							env, err = commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
								Name:    task.ProjectName,
								EnvName: jobTaskSpec.Env,
							})
							if err != nil {
								log.Errorf("find project error: %v", err)
								return
							}

							kubeClient, err = clientmanager.NewKubeClientManager().GetControllerRuntimeClient(env.ClusterID)
							if err != nil {
								log.Errorf("can't init k8s client: %v", err)
								return
							}

							timeout := time.After(time.Duration(jobTaskSpec.Timeout) * time.Second)
							jobTaskSpec.ReplaceResources, err = runtimeJobController.GetResourcesPodOwnerUID(kubeClient, env.Namespace, nil, nil, jobTaskSpec.ReplaceResources)
							if err != nil {
								log.Errorf("failed to get resources pod owner uid, error: %s", err)
								return
							}
							status, err = runtimeJobController.CheckDeployStatus(context.TODO(), kubeClient, env.Namespace, jobTaskSpec.RelatedPodLabels, jobTaskSpec.ReplaceResources, timeout, logger)
							if err != nil {
								log.Errorf("failed to check deploy status, error: %s", err)
							}
						}
					}()

					return nil
				case string(config.JobZadigHelmDeploy):
					err = commonutil.CheckZadigProfessionalLicense()
					if err != nil {
						return err
					}

					inputSpec := new(CommonRevertInput)
					err = commonmodels.IToi(input, inputSpec)
					if err != nil {
						return fmt.Errorf("failed to decode deploy revert job spec, error: %s", err)
					}

					jobTaskSpec := &commonmodels.JobTaskHelmDeploySpec{}
					if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
						logger.Error(err)
						return fmt.Errorf("failed to decode nacos job spec, error: %s", err)
					}

					job.Reverted = true
					task.Reverted = true
					err = commonrepo.NewworkflowTaskv4Coll().Update(task.ID.Hex(), task)
					if err != nil {
						err = fmt.Errorf("failed to update nacos job revert information, error: %s", err)
						log.Error(err)
						return err
					}

					envSvcVersionYaml, err := commonservice.GetEnvServiceVersionYaml(ctx, task.ProjectName, jobTaskSpec.Env, jobTaskSpec.ServiceName, jobTaskSpec.OriginRevision, false, jobTaskSpec.IsProduction, logger)
					if err != nil {
						err = fmt.Errorf("failed to get env service version yaml, error: %s", err)
						log.Error(err)
						return err
					}
					revertSpec := &commonmodels.JobTaskDeployRevertSpec{
						Env:                jobTaskSpec.Env,
						ServiceName:        jobTaskSpec.ServiceName,
						ServiceType:        jobTaskSpec.ServiceType,
						Production:         jobTaskSpec.IsProduction,
						Yaml:               envSvcVersionYaml.Yaml,
						Containers:         envSvcVersionYaml.Containers,
						VariableYaml:       envSvcVersionYaml.VariableYaml,
						OverrideKVs:        envSvcVersionYaml.OverrideKVs,
						Revision:           jobTaskSpec.OriginRevision,
						RevisionCreateTime: envSvcVersionYaml.CreateTime,
						JobTaskCommonRevertSpec: commonmodels.JobTaskCommonRevertSpec{
							Detail: inputSpec.Detail,
						},
					}

					rollbackStatus, err := commonservice.RollbackEnvServiceVersion(ctx, task.ProjectName, jobTaskSpec.Env, jobTaskSpec.ServiceName, jobTaskSpec.OriginRevision, false, jobTaskSpec.IsProduction, inputSpec.Detail, logger)
					if err != nil {
						log.Errorf("failed to rollback env service version, error: %s", err)
						return err
					}

					revert := &commonmodels.WorkflowTaskRevert{
						TaskID:        taskID,
						WorkflowName:  workflowName,
						JobName:       jobName,
						RevertSpec:    revertSpec,
						CreateTime:    time.Now().Unix(),
						TaskCreator:   userName,
						TaskCreatorID: userID,
						Status:        config.StatusRunning,
					}
					revertID, err := commonrepo.NewWorkflowTaskRevertColl().Create(revert)
					if err != nil {
						log.Warnf("failed to insert revert task logs, error: %s", err)
					}

					if jobTaskSpec.Timeout == 0 {
						jobTaskSpec.Timeout = setting.DeployTimeout
					}

					go func() {
						var (
							err    error
							status = config.StatusFailed
						)
						defer func() {
							err = commonrepo.NewWorkflowTaskRevertColl().UpateStatusByID(revertID, status)
							if err != nil {
								log.Errorf("failed to update revert task status, error: %s", err)
							}
						}()

						select {
						case result := <-rollbackStatus.HelmDeployStatusChan:
							if !result {
								status = config.StatusFailed
							}
							status = config.StatusPassed
							break
						case <-time.After(time.Second*time.Duration(jobTaskSpec.Timeout) + time.Minute):
							log.Errorf("failed to upgrade relase for service: %s, timeout", jobTaskSpec.ServiceName)
							status = config.StatusTimeout
						}
					}()

					return nil
				case string(config.JobApollo):
					jobTaskSpec := &commonmodels.JobTaskApolloSpec{}
					if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
						logger.Error(err)
						return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
					}

					inputSpec := new(ApolloRevertInput)
					err = commonmodels.IToi(input, inputSpec)
					if err != nil {
						return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
					}

					info, err := mongodb.NewConfigurationManagementColl().GetApolloByID(context.Background(), jobTaskSpec.ApolloID)
					if err != nil {
						return fmt.Errorf("failed to get apollo info, error: %s", err)
					}

					link := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
						config2.SystemAddress(),
						task.ProjectName,
						task.WorkflowName,
						task.TaskID,
						url.QueryEscape(task.WorkflowDisplayName))

					var fail bool
					revertDatas := make([]*commonmodels.JobTaskApolloNamespace, 0)

					client := apollo.NewClient(info.ServerAddress, info.Token)
					for _, namespace := range inputSpec.ApolloDatas {
						revertData := &commonmodels.JobTaskApolloNamespace{
							ApolloNamespace: commonmodels.ApolloNamespace{
								AppID:      namespace.AppID,
								Env:        namespace.Env,
								ClusterID:  namespace.ClusterID,
								Namespace:  namespace.Namespace,
								Type:       namespace.Type,
								KeyValList: namespace.KeyValList,
							},
						}
						revertDatas = append(revertDatas, revertData)

						for _, kv := range namespace.KeyValList {
							err := client.UpdateKeyVal(namespace.AppID, namespace.Env, namespace.ClusterID, namespace.Namespace, kv.Key, kv.Val, info.ApolloAuthConfig.User)
							if err != nil {
								fail = true
								revertData.Error = fmt.Sprintf("update error: %v", err)
								continue
							}

						}
						err := client.Release(namespace.AppID, namespace.Env, namespace.ClusterID, namespace.Namespace,
							&apollo.ReleaseArgs{
								ReleaseTitle:   time.Now().Format("20060102150405") + "-zadig",
								ReleaseComment: fmt.Sprintf("工作流 %s 回滚\n详情: %s", task.WorkflowDisplayName, link),
								ReleasedBy:     info.ApolloAuthConfig.User,
							})
						if err != nil {
							fail = true
							revertData.Error = fmt.Sprintf("release error: %v", err)
						}
					}

					if fail {
						return fmt.Errorf("some errors occurred in revert apollo job")
					}

					job.Reverted = true
					task.Reverted = true
					err = commonrepo.NewworkflowTaskv4Coll().Update(task.ID.Hex(), task)
					if err != nil {
						log.Errorf("failed to update apollo job revert information, error: %s", err)
					}

					revertTaskSpec := &commonmodels.JobTaskApolloSpec{
						ApolloID:      jobTaskSpec.ApolloID,
						NamespaceList: revertDatas,
						JobTaskCommonRevertSpec: commonmodels.JobTaskCommonRevertSpec{
							Detail: inputSpec.Detail,
						},
					}

					_, err = commonrepo.NewWorkflowTaskRevertColl().Create(&commonmodels.WorkflowTaskRevert{
						TaskID:        taskID,
						WorkflowName:  workflowName,
						JobName:       jobName,
						RevertSpec:    revertTaskSpec,
						CreateTime:    time.Now().Unix(),
						TaskCreator:   userName,
						TaskCreatorID: userID,
						Status:        config.StatusPassed,
					})
					if err != nil {
						log.Warnf("failed to insert revert task logs, error: %s", err)
					}

					return nil
				case string(config.JobNacos):
					jobTaskSpec := &commonmodels.JobTaskNacosSpec{}
					if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
						logger.Error(err)
						return fmt.Errorf("failed to decode nacos job spec, error: %s", err)
					}

					inputSpec := new(NacosRevertInput)
					err = commonmodels.IToi(input, inputSpec)
					if err != nil {
						return fmt.Errorf("failed to decode nacos job spec, error: %s", err)
					}

					err = revertNacosJob(jobTaskSpec, inputSpec.NacosDatas)
					if err != nil {
						log.Errorf("failed to revert nacos job %s, error: %s", job.Name, err)
						return fmt.Errorf("failed to revert nacos job: %s, error: %s", job.Name, err)
					}

					job.Reverted = true
					task.Reverted = true
					err = commonrepo.NewworkflowTaskv4Coll().Update(task.ID.Hex(), task)
					if err != nil {
						log.Errorf("failed to update nacos job revert information, error: %s", err)
					}

					inputData := make([]*commonmodels.NacosData, 0)
					client, err := nacos.NewNacosClient(jobTaskSpec.Type, jobTaskSpec.NacosAddr, jobTaskSpec.AuthConfig)
					if err != nil {
						return err
					}

					for _, in := range inputSpec.NacosDatas {
						originalConfig, err := client.GetConfig(in.DataID, in.Group, in.NamespaceID)
						if err != nil {
							log.Errorf("failed to find current config for data: %s in namespace: %s, error: %s", in.DataID, in.NamespaceID, err)
							return fmt.Errorf("failed to find current config for data: %s in namespace: %s, error: %s", in.DataID, in.NamespaceID, err)
						}
						nacosID := types.NacosDataID{
							DataID: in.DataID,
							Group:  in.Group,
						}
						inputData = append(inputData, &commonmodels.NacosData{
							NacosConfig: types.NacosConfig{
								NacosDataID:     nacosID,
								Format:          in.Format,
								Content:         in.Content,
								OriginalContent: originalConfig.Content,
								NamespaceID:     in.NamespaceID,
								NamespaceName:   in.NamespaceName,
							},
						})
					}

					revertTaskSpec := &commonmodels.JobTaskNacosSpec{
						NacosID:       jobTaskSpec.NacosID,
						NamespaceID:   jobTaskSpec.NamespaceID,
						NamespaceName: jobTaskSpec.NamespaceName,
						NacosAddr:     jobTaskSpec.NacosAddr,
						Type:          jobTaskSpec.Type,
						AuthConfig:    jobTaskSpec.AuthConfig,
						NacosDatas:    inputData,
						JobTaskCommonRevertSpec: commonmodels.JobTaskCommonRevertSpec{
							Detail: inputSpec.Detail,
						},
					}

					_, err = commonrepo.NewWorkflowTaskRevertColl().Create(&commonmodels.WorkflowTaskRevert{
						TaskID:        taskID,
						WorkflowName:  workflowName,
						JobName:       jobName,
						RevertSpec:    revertTaskSpec,
						CreateTime:    time.Now().Unix(),
						TaskCreator:   userName,
						TaskCreatorID: userID,
						Status:        config.StatusPassed,
					})

					if err != nil {
						log.Warnf("failed to insert revert task logs, error: %s", err)
					}

					return nil
				case string(config.JobSQL):
					jobTaskSpec := &commonmodels.JobTaskSQLSpec{}
					if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
						logger.Error(err)
						return fmt.Errorf("failed to decode nacos job spec, error: %s", err)
					}
					inputSpec := new(SQLRevertInput)
					err = commonmodels.IToi(input, inputSpec)
					if err != nil {
						return fmt.Errorf("failed to decode sql revert job spec, error: %s", err)
					}

					info, err := mongodb.NewDBInstanceColl().Find(&mongodb.DBInstanceCollFindOption{Id: jobTaskSpec.ID})
					if err != nil {
						return fmt.Errorf("failed to find database info to run the rollback sql, error: %s", err)
					}

					job.Reverted = true
					task.Reverted = true

					err = commonrepo.NewworkflowTaskv4Coll().Update(task.ID.Hex(), task)
					if err != nil {
						log.Errorf("failed to update sql job revert information, error: %s", err)
					}

					revertTaskSpec := &commonmodels.JobTaskSQLSpec{
						ID:      jobTaskSpec.ID,
						Type:    jobTaskSpec.Type,
						SQL:     inputSpec.SQL,
						Results: make([]*commonmodels.SQLExecResult, 0),
						JobTaskCommonRevertSpec: commonmodels.JobTaskCommonRevertSpec{
							Detail: inputSpec.Detail,
						},
					}

					db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8&multiStatements=true", info.Username, info.Password, info.Host, info.Port))
					if err != nil {
						return errors.Errorf("failed to run rollback sql, connect db error: %v", err)
					}
					defer db.Close()

					sqls := strings.SplitAfter(inputSpec.SQL, ";")
					for _, sql := range sqls {
						if sql == "" {
							continue
						}

						execResult := &commonmodels.SQLExecResult{}

						execResult.SQL = strings.TrimSpace(sql)
						execResult.Status = setting.SQLExecStatusNotExec

						revertTaskSpec.Results = append(revertTaskSpec.Results, execResult)
					}

					for _, execResult := range revertTaskSpec.Results {
						now := time.Now()
						result, err := db.Exec(execResult.SQL)
						if err != nil {
							execResult.Status = setting.SQLExecStatusFailed
							_, err = commonrepo.NewWorkflowTaskRevertColl().Create(&commonmodels.WorkflowTaskRevert{
								TaskID:        taskID,
								WorkflowName:  workflowName,
								JobName:       jobName,
								RevertSpec:    revertTaskSpec,
								CreateTime:    time.Now().Unix(),
								TaskCreator:   userName,
								TaskCreatorID: userID,
								Status:        config.StatusFailed,
							})

							if err != nil {
								log.Warnf("failed to insert revert task logs, error: %s", err)
							}

							return fmt.Errorf("exec SQL \"%s\" error: %v", execResult.SQL, err)
						}
						execResult.Status = setting.SQLExecStatusSuccess
						execResult.ElapsedTime = time.Now().Sub(now).Milliseconds()

						rowsAffected, err := result.RowsAffected()
						if err != nil {
							return fmt.Errorf("get affect rows error: %v", err)
						}
						execResult.RowsAffected = rowsAffected
					}

					_, err = commonrepo.NewWorkflowTaskRevertColl().Create(&commonmodels.WorkflowTaskRevert{
						TaskID:        taskID,
						WorkflowName:  workflowName,
						JobName:       jobName,
						RevertSpec:    revertTaskSpec,
						CreateTime:    time.Now().Unix(),
						TaskCreator:   userName,
						TaskCreatorID: userID,
						Status:        config.StatusPassed,
					})

					if err != nil {
						log.Warnf("failed to insert revert task logs, error: %s", err)
					}

					return nil
				default:
					return fmt.Errorf("job of type: %s does not support reverting yet", job.JobType)
				}
			}
		}
	}

	return fmt.Errorf("failed to revert job: %s, job not found", jobName)
}

func GetWorkflowTaskV4JobRevert(workflowName, jobName string, taskID int64, logger *zap.SugaredLogger) (interface{}, error) {
	revertInfo, err := commonrepo.NewWorkflowTaskRevertColl().List(&commonrepo.ListWorkflowRevertOption{
		TaskID:       taskID,
		WorkflowName: workflowName,
		JobName:      jobName,
	})
	if err != nil {
		logger.Errorf("failed to list job revert info for job: %s in workflow: %s(%d), error: %s", jobName, workflowName, taskID, err)
		return nil, fmt.Errorf("failed to list job revert info for job: %s in workflow: %s(%d), error: %s", jobName, workflowName, taskID, err)
	}

	return revertInfo, nil
}

func revertNacosJob(jobspec *commonmodels.JobTaskNacosSpec, input []*commonmodels.NacosData) error {
	client, err := nacos.NewNacosClient(jobspec.Type, jobspec.NacosAddr, jobspec.AuthConfig)
	if err != nil {
		return err
	}

	for _, data := range input {
		if err := client.UpdateConfig(data.DataID, data.Group, jobspec.NamespaceID, data.Content, data.Format); err != nil {
			return err
		}
	}

	return nil
}

type TaskHistoryFilter struct {
	PageSize     int64  `json:"page_size"    form:"page_size,default=20"`
	PageNum      int64  `json:"page_num"     form:"page_num,default=1"`
	WorkflowName string `json:"workflow_name" form:"workflow_name"`
	ProjectName  string `json:"projectName"  form:"projectName"`
	QueryType    string `json:"queryType"    form:"queryType"`
	Filters      string `json:"filters" form:"filters"`
	JobName      string `json:"jobName" form:"jobName"`
}

func ListWorkflowTaskV4ByFilter(filter *TaskHistoryFilter, filterList []string, logger *zap.SugaredLogger) ([]*commonmodels.WorkflowTaskPreview, int64, error) {
	var listTaskOpt *mongodb.WorkFlowTaskFilter
	switch filter.QueryType {
	case "creator":
		listTaskOpt = &mongodb.WorkFlowTaskFilter{
			WorkflowName: filter.WorkflowName,
			ProjectName:  filter.ProjectName,
			Creator:      filterList,
		}
	case "serviceName":
		listTaskOpt = &mongodb.WorkFlowTaskFilter{
			JobName:      filter.JobName,
			WorkflowName: filter.WorkflowName,
			ProjectName:  filter.ProjectName,
			Service:      filterList,
		}
	case "taskStatus":
		listTaskOpt = &mongodb.WorkFlowTaskFilter{
			WorkflowName: filter.WorkflowName,
			ProjectName:  filter.ProjectName,
			Status:       filterList,
		}
	case "envName":
		listTaskOpt = &mongodb.WorkFlowTaskFilter{
			JobName:      filter.JobName,
			WorkflowName: filter.WorkflowName,
			ProjectName:  filter.ProjectName,
			Env:          filterList,
		}
	default:
		listTaskOpt = &mongodb.WorkFlowTaskFilter{
			WorkflowName: filter.WorkflowName,
			ProjectName:  filter.ProjectName,
		}
	}
	tasks, total, err := commonrepo.NewworkflowTaskv4Coll().ListByFilter(listTaskOpt, filter.PageNum, filter.PageSize)
	if err != nil {
		logger.Errorf("list workflowTaskV4 error: %s", err)
		return nil, total, err
	}

	envMap := make(map[string]*commonmodels.Product)
	taskPreviews := make([]*commonmodels.WorkflowTaskPreview, 0)
	for _, task := range tasks {
		preview := &commonmodels.WorkflowTaskPreview{
			TaskID:              task.TaskID,
			TaskCreator:         task.TaskCreator,
			ProjectName:         task.ProjectName,
			WorkflowName:        task.WorkflowName,
			WorkflowDisplayName: task.WorkflowDisplayName,
			Remark:              task.Remark,
			Status:              task.Status,
			Reverted:            task.Reverted,
			CreateTime:          task.CreateTime,
			StartTime:           task.StartTime,
			EndTime:             task.EndTime,
			Hash:                task.Hash,
		}

		stagePreviews := make([]*commonmodels.StagePreview, 0)
		for _, stage := range task.WorkflowArgs.Stages {
			stagePreview := &commonmodels.StagePreview{
				Name: stage.Name,
			}
			for _, job := range stage.Jobs {
				if job.Skipped {
					continue
				}
				jobPreview := &commonmodels.JobPreview{
					Name:    job.Name,
					JobType: string(job.JobType),
				}
				switch job.JobType {
				case config.JobZadigBuild:
					build := new(commonmodels.ZadigBuildJobSpec)
					if err := commonmodels.IToi(job.Spec, build); err != nil {
						return nil, 0, err
					}
					serviceModules := make([]*commonmodels.WorkflowServiceModule, 0)
					for _, serviceAndBuild := range build.ServiceAndBuilds {
						sm := &commonmodels.WorkflowServiceModule{
							ServiceWithModule: commonmodels.ServiceWithModule{
								ServiceName:   serviceAndBuild.ServiceName,
								ServiceModule: serviceAndBuild.ServiceModule,
							},
						}
						for _, repo := range serviceAndBuild.Repos {
							sm.CodeInfo = append(sm.CodeInfo, repo)
						}
						serviceModules = append(serviceModules, sm)
					}
					jobPreview.ServiceModules = serviceModules
				case config.JobZadigDeploy:
					deploy := new(commonmodels.ZadigDeployJobSpec)
					if err := commonmodels.IToi(job.Spec, deploy); err != nil {
						return nil, 0, err
					}
					serviceModules := make([]*commonmodels.WorkflowServiceModule, 0)
					for _, svc := range deploy.Services {
						for _, module := range svc.Modules {
							sm := &commonmodels.WorkflowServiceModule{
								ServiceWithModule: commonmodels.ServiceWithModule{
									ServiceName:   svc.ServiceName,
									ServiceModule: module.ServiceModule,
								},
							}
							serviceModules = append(serviceModules, sm)
						}
					}
					jobPreview.ServiceModules = serviceModules
					jobPreview.Envs = &commonmodels.WorkflowEnv{
						EnvName:    deploy.Env,
						Production: deploy.Production,
						EnvAlias:   commonutil.GetEnvAlias(commonutil.GetEnvInfoNoErr(filter.ProjectName, deploy.Env, envMap)),
					}
				case config.JobZadigTesting:
					test := new(commonmodels.ZadigTestingJobSpec)
					if err := commonmodels.IToi(job.Spec, test); err != nil {
						return nil, 0, err
					}

					serviceModules := make([]*commonmodels.WorkflowServiceModule, 0)
					for _, service := range test.ServiceAndTests {
						sm := &commonmodels.WorkflowServiceModule{
							ServiceWithModule: commonmodels.ServiceWithModule{
								ServiceName:   service.ServiceName,
								ServiceModule: service.ServiceModule,
							},
						}
						for _, repo := range service.Repos {
							sm.CodeInfo = append(sm.CodeInfo, repo)
						}
						serviceModules = append(serviceModules, sm)
					}
					jobPreview.ServiceModules = serviceModules

					// get test report
					testModules := make([]*commonmodels.WorkflowTestModule, 0)
					testResultList, err := commonrepo.NewCustomWorkflowTestReportColl().ListByWorkflowJobName(filter.WorkflowName, job.Name, task.TaskID)
					if err != nil {
						log.Errorf("failed to list junit test report for workflow: %s, error: %s", filter.WorkflowName, err)
						return nil, 0, fmt.Errorf("failed to list junit test report for workflow: %s, error: %s", filter.WorkflowName, err)
					}

					for _, testResult := range testResultList {
						testModules = append(testModules, &commonmodels.WorkflowTestModule{
							JobName:        job.Name,
							JobTaskName:    testResult.JobTaskName,
							Type:           "function",
							TestName:       testResult.ZadigTestName,
							TestCaseNum:    testResult.TestCaseNum,
							SuccessCaseNum: testResult.SuccessCaseNum,
							TestTime:       testResult.TestTime,
						})
					}
					jobPreview.TestModules = testModules
				case config.JobZadigDistributeImage:
					distribute := new(commonmodels.ZadigDistributeImageJobSpec)
					if err := commonmodels.IToi(job.Spec, distribute); err != nil {
						return nil, 0, err
					}
					serviceModules := make([]*commonmodels.WorkflowServiceModule, 0)
					for _, target := range distribute.Targets {
						sm := &commonmodels.WorkflowServiceModule{
							ServiceWithModule: commonmodels.ServiceWithModule{
								ServiceName:   target.ServiceName,
								ServiceModule: target.ServiceModule,
							},
						}
						serviceModules = append(serviceModules, sm)
					}
				}
				stagePreview.Jobs = append(stagePreview.Jobs, jobPreview)
			}
			if len(stagePreview.Jobs) > 0 {
				stagePreviews = append(stagePreviews, stagePreview)
			}
		}

		for _, stage := range task.Stages {
			for _, stagePreview := range stagePreviews {
				if stagePreview.Name == stage.Name {
					stagePreview.Status = stage.Status
					stagePreview.StartTime = stage.StartTime
					stagePreview.EndTime = stage.EndTime
					stagePreview.ManualExec = stage.ManualExec
					stagePreview.Parallel = stage.Parallel
					stagePreview.Error = stage.Error
					break
				}
			}
		}
		preview.Stages = stagePreviews
		taskPreviews = append(taskPreviews, preview)
	}
	cleanWorkflowV4TasksPreviews(taskPreviews)
	return taskPreviews, total, nil
}

// clean extra message for list workflow
func cleanWorkflowV4TasksPreviews(workflows []*commonmodels.WorkflowTaskPreview) {
	const StatusNotRun = ""
	for _, workflow := range workflows {
		var stageList []*commonmodels.StagePreview
		workflow.WorkflowArgs = nil
		for _, stage := range workflow.Stages {
			stageList = append(stageList, stage)
		}
		workflow.Stages = stageList
	}
}

func getLatestWorkflowTaskV4(workflowName string) (*commonmodels.WorkflowTask, error) {
	resp, err := commonrepo.NewworkflowTaskv4Coll().GetLatest(workflowName)
	if err != nil {
		return nil, err
	}
	resp.WorkflowArgs = nil
	resp.OriginWorkflowArgs = nil
	resp.Stages = nil
	return resp, nil
}

func CancelWorkflowTaskV4(userName, workflowName string, taskID int64, logger *zap.SugaredLogger) error {
	if err := runtimeWorkflowController.CancelWorkflowTask(userName, workflowName, taskID, logger); err != nil {
		logger.Errorf("cancel workflowTaskV4 error: %s", err)
		return e.ErrCancelTask.AddErr(err)
	}
	return nil
}

func GetWorkflowTaskV4(workflowName string, taskID int64, logger *zap.SugaredLogger) (*WorkflowTaskPreview, error) {
	task, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		logger.Errorf("find workflowTaskV4 error: %s", err)
		return nil, err
	}
	resp := &WorkflowTaskPreview{
		TaskID:              task.TaskID,
		WorkflowName:        task.WorkflowName,
		WorkflowDisplayName: task.WorkflowDisplayName,
		ProjectName:         task.ProjectName,
		Remark:              task.Remark,
		Status:              task.Status,
		Reverted:            task.Reverted,
		Params:              task.Params,
		TaskCreator:         task.TaskCreator,
		TaskRevoker:         task.TaskRevoker,
		CreateTime:          task.CreateTime,
		StartTime:           task.StartTime,
		EndTime:             task.EndTime,
		Error:               task.Error,
		IsRestart:           task.IsRestart,
		Debug:               task.IsDebug,
		ApprovalTicketID:    task.ApprovalTicketID,
		ApprovalID:          task.ApprovalID,
	}
	timeNow := time.Now().Unix()
	for _, stage := range task.Stages {
		resp.Stages = append(resp.Stages, &StageTaskPreview{
			Name:       stage.Name,
			Status:     stage.Status,
			StartTime:  stage.StartTime,
			EndTime:    stage.EndTime,
			Parallel:   stage.Parallel,
			ManualExec: stage.ManualExec,
			Jobs:       jobsToJobPreviews(stage.Jobs, task.GlobalContext, timeNow, task.ProjectName),
			Error:      stage.Error,
		})
	}
	return resp, nil
}

func ApproveStage(workflowName, jobName, userName, userID, comment string, taskID int64, approve bool, logger *zap.SugaredLogger) error {
	if workflowName == "" || jobName == "" || taskID == 0 {
		errMsg := fmt.Sprintf("can not find approved workflow: %s, taskID: %d,jobName: %s", workflowName, taskID, jobName)
		logger.Error(errMsg)
		return e.ErrApproveTask.AddDesc(errMsg)
	}
	if err := runtimeWorkflowController.ApproveStage(workflowName, jobName, userName, userID, comment, taskID, approve); err != nil {
		logger.Error(err)
		return e.ErrApproveTask.AddErr(err)
	}
	return nil
}

func HandleJobError(workflowName, jobName, userID, username string, taskID int64, decision workflowtool.JobErrorDecision, logger *zap.SugaredLogger) error {
	if workflowName == "" || jobName == "" || taskID == 0 {
		errMsg := fmt.Sprintf("can not find approved workflow: %s, taskID: %d,jobName: %s", workflowName, taskID, jobName)
		logger.Error(errMsg)
		return e.ErrApproveTask.AddDesc(errMsg)
	}
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		errMsg := fmt.Sprintf("can not find workflow task: %s, taskID: %d to handle its error, err: %s", workflowName, taskID, err)
		logger.Error(errMsg)
		return e.ErrApproveTask.AddDesc(errMsg)
	}

	found := false
	var errorJob *commonmodels.JobTask
	for _, stage := range workflowTask.Stages {
		if found {
			break
		}
		for _, job := range stage.Jobs {
			if job.Name == jobName {
				found = true
				errorJob = job
				break
			}
		}
	}

	if !found {
		errMsg := fmt.Sprintf("can not find job %s in workflow task: %s, taskID: %d to handle its error, err: %s", jobName, workflowName, taskID, err)
		logger.Error(errMsg)
		return e.ErrApproveTask.AddDesc(errMsg)
	}

	if errorJob.ErrorPolicy == nil || errorJob.ErrorPolicy.Policy != config.JobErrorPolicyManualCheck {
		errMsg := fmt.Sprintf("error policy for job: %s is %s", jobName, errorJob.ErrorPolicy.Policy)
		logger.Error(errMsg)
		return e.ErrApproveTask.AddDesc(errMsg)
	}

	_, userMap := util.GeneFlatUsersWithCaller(errorJob.ErrorPolicy.ApprovalUsers, userID)

	if _, ok := userMap[userID]; !ok {
		errMsg := fmt.Sprintf("user %s is not authorized to perform error handling", username)
		logger.Error(errMsg)
		return e.ErrApproveTask.AddDesc(errMsg)
	}

	if err := workflowtool.SetJobErrorHandlingDecision(workflowName, jobName, taskID, decision, userID, username); err != nil {
		logger.Error(err)
		return e.ErrApproveTask.AddErr(err)
	}
	return nil
}

func jobsToJobPreviews(jobs []*commonmodels.JobTask, context map[string]string, now int64, projectName string) []*JobTaskPreview {
	envMap := make(map[string]*commonmodels.Product)
	resp := []*JobTaskPreview{}

	for _, job := range jobs {
		costSeconds := int64(0)
		if job.StartTime != 0 {
			costSeconds = now - job.StartTime
			if job.EndTime != 0 {
				costSeconds = job.EndTime - job.StartTime
			}
		}
		jobPreview := &JobTaskPreview{
			Name:                 job.Name,
			Key:                  job.Key,
			Reverted:             job.Reverted,
			OriginName:           job.OriginName,
			DisplayName:          job.DisplayName,
			Status:               job.Status,
			StartTime:            job.StartTime,
			EndTime:              job.EndTime,
			Error:                job.Error,
			JobType:              job.JobType,
			BreakpointBefore:     job.BreakpointBefore,
			BreakpointAfter:      job.BreakpointAfter,
			CostSeconds:          costSeconds,
			JobInfo:              job.JobInfo,
			ErrorPolicy:          job.ErrorPolicy,
			ErrorHandlerUserID:   job.ErrorHandlerUserID,
			ErrorHandlerUserName: job.ErrorHandlerUserName,
			RetryCount:           job.RetryCount,
		}
		switch job.JobType {
		case string(config.JobFreestyle):
			fallthrough
		case string(config.JobZadigBuild):
			spec := ZadigBuildJobSpec{}
			taskJobSpec := &commonmodels.JobTaskFreestyleSpec{}
			if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
				continue
			}
			for _, arg := range taskJobSpec.Properties.Envs {
				if arg.Key == "SERVICE_NAME" {
					spec.ServiceName = arg.Value
					continue
				}
				if arg.Key == "SERVICE_MODULE" {
					spec.ServiceModule = arg.Value
					continue
				}
			}

			// get from global context
			imageContextKey := runtimeWorkflowController.GetContextKey(jobspec.GetJobOutputKey(job.Key, "IMAGE"))
			if context != nil {
				spec.Image = context[imageContextKey]
			}

			spec.Envs = taskJobSpec.Properties.CustomEnvs
			for _, step := range taskJobSpec.Steps {
				if step.StepType == config.StepGit {
					stepSpec := &stepspec.StepGitSpec{}
					commonmodels.IToi(step.Spec, &stepSpec)
					if spec.Repos == nil {
						spec.Repos = make([]*types.Repository, 0)
					}
					spec.Repos = append(spec.Repos, stepSpec.Repos...)
					continue
				} else if step.StepType == config.StepPerforce {
					stepSpec := &stepspec.StepP4Spec{}
					commonmodels.IToi(step.Spec, &stepSpec)
					if spec.Repos == nil {
						spec.Repos = make([]*types.Repository, 0)
					}
					spec.Repos = append(spec.Repos, stepSpec.Repos...)
					continue
				}
				if step.StepType == config.StepArchive && strings.HasSuffix(step.Name, "-pkgfile-archive") {
					stepSpec := &stepspec.StepArchiveSpec{}
					if err := commonmodels.IToi(step.Spec, &stepSpec); err != nil {
						continue
					}

					if len(stepSpec.UploadDetail) > 0 {
						spec.Package = stepSpec.UploadDetail[len(stepSpec.UploadDetail)-1].DestinationPath + "/" + stepSpec.UploadDetail[len(stepSpec.UploadDetail)-1].Name
					}
				}
			}
			jobPreview.Spec = spec
		case string(config.JobZadigDistributeImage):
			spec := &DistributeImageJobSpec{}
			taskJobSpec := &commonmodels.JobTaskFreestyleSpec{}
			if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
				continue
			}

			for _, step := range taskJobSpec.Steps {
				if step.StepType == config.StepDistributeImage {
					stepSpec := &stepspec.StepImageDistributeSpec{}
					commonmodels.IToi(step.Spec, &stepSpec)
					spec.DistributeTarget = stepSpec.DistributeTarget
					break
				}
			}
			jobPreview.Spec = spec
		case string(config.JobZadigTesting):
			spec := &ZadigTestingJobSpec{}
			jobPreview.Spec = spec
			taskJobSpec := &commonmodels.JobTaskFreestyleSpec{}
			if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
				continue
			}
			spec.Envs = taskJobSpec.Properties.CustomEnvs
			for _, step := range taskJobSpec.Steps {
				if step.StepType == config.StepGit {
					stepSpec := &stepspec.StepGitSpec{}
					commonmodels.IToi(step.Spec, &stepSpec)
					if spec.Repos == nil {
						spec.Repos = make([]*types.Repository, 0)
					}
					spec.Repos = append(spec.Repos, stepSpec.Repos...)
					continue
				} else if step.StepType == config.StepPerforce {
					stepSpec := &stepspec.StepP4Spec{}
					commonmodels.IToi(step.Spec, &stepSpec)
					if spec.Repos == nil {
						spec.Repos = make([]*types.Repository, 0)
					}
					spec.Repos = append(spec.Repos, stepSpec.Repos...)
					continue
				}
			}
			for _, arg := range taskJobSpec.Properties.Envs {
				if arg.Key == "TESTING_PROJECT" {
					spec.ProjectName = arg.Value
					continue
				}
				if arg.Key == "TESTING_NAME" {
					spec.TestName = arg.Value
					continue
				}
				if arg.Key == "TESTING_TYPE" {
					spec.TestType = arg.Value
					continue
				}
				if arg.Key == "SERVICE_NAME" {
					spec.ServiceName = arg.Value
					continue
				}
				if arg.Key == "SERVICE_MODULE" {
					spec.ServiceModule = arg.Value
					continue
				}

			}
			if job.Status == config.StatusPassed || job.Status == config.StatusFailed ||
				job.Status == config.StatusUnstable || job.Status == config.StatusManualApproval {
				for _, step := range taskJobSpec.Steps {
					if step.Name == config.TestJobArchiveResultStepName {
						spec.Archive = true
					}
					if step.Name == config.TestJobHTMLReportStepName {
						spec.HtmlReport = true
					}
					if step.Name == config.TestJobJunitReportStepName {
						spec.JunitReport = true
					}
				}
			}
		case string(config.JobZadigScanning):
			spec := ZadigScanningJobSpec{}
			taskJobSpec := &commonmodels.JobTaskFreestyleSpec{}
			if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
				continue
			}
			spec.Envs = taskJobSpec.Properties.CustomEnvs
			for _, step := range taskJobSpec.Steps {
				if step.Name == config.ScanningJobArchiveResultStepName {
					spec.IsHasArtifact = true
				}
				if step.StepType == config.StepGit {
					stepSpec := &stepspec.StepGitSpec{}
					commonmodels.IToi(step.Spec, &stepSpec)
					if spec.Repos == nil {
						spec.Repos = make([]*types.Repository, 0)
					}
					spec.Repos = append(spec.Repos, stepSpec.Repos...)
					continue
				}

				if step.StepType == config.StepPerforce {
					stepSpec := &stepspec.StepP4Spec{}
					commonmodels.IToi(step.Spec, &stepSpec)
					if spec.Repos == nil {
						spec.Repos = make([]*types.Repository, 0)
					}
					spec.Repos = append(spec.Repos, stepSpec.Repos...)
					continue
				}

				if step.StepType == config.StepSonarGetMetrics {
					stepSpec := &stepspec.StepSonarGetMetricsSpec{}
					commonmodels.IToi(step.Spec, &stepSpec)
					spec.SonarMetrics = stepSpec.SonarMetrics
					continue
				}
			}
			sonarURL := ""
			for _, arg := range taskJobSpec.Properties.Envs {
				if arg.Key == "SONAR_LINK" {
					spec.LinkURL = arg.Value
					continue
				}
				if arg.Key == "SCANNING_NAME" {
					spec.ScanningName = arg.Value
					continue
				}

				if arg.Key == "SCANNING_TYPE" {
					spec.TestType = arg.Value
					continue
				}
				if arg.Key == "SERVICE_NAME" {
					spec.ServiceName = arg.Value
					continue
				}
				if arg.Key == "SERVICE_MODULE" {
					spec.ServiceModule = arg.Value
					continue
				}
				if arg.Key == "SONAR_URL" {
					sonarURL = arg.Value
					continue
				}
			}
			if sonarURL != "" {
				projectKey := ""
				projectScanningOutputKey := jobspec.GetJobOutputKey(job.Key, setting.WorkflowScanningJobOutputKeyProject)
				projectScanningOutputKey = runtimeWorkflowController.GetContextKey(projectScanningOutputKey)
				if context[projectScanningOutputKey] != "" {
					projectKey = context[projectScanningOutputKey]
				}

				branch := ""
				branchScanningOutputKey := jobspec.GetJobOutputKey(job.Key, setting.WorkflowScanningJobOutputKeyBranch)
				branchScanningOutputKey = runtimeWorkflowController.GetContextKey(branchScanningOutputKey)
				if context[branchScanningOutputKey] != "" {
					branch = context[branchScanningOutputKey]
				}

				resultAddr, err := sonar.GetSonarAddress(sonarURL, projectKey, branch)
				if err != nil {
					log.Errorf("failed to get sonar address with project key %s, error: %v", projectKey, err)
					continue
				}
				spec.LinkURL = resultAddr
			} else {
				log.Errorf("failed to get sonar url from job task's env")
			}
			jobPreview.Spec = spec
		case string(config.JobZadigDeploy):
			spec := ZadigDeployJobPreviewSpec{}
			taskJobSpec := &commonmodels.JobTaskDeploySpec{}
			if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
				continue
			}
			spec.Env = taskJobSpec.Env
			spec.Production = commonutil.GetEnvProduction(commonutil.GetEnvInfoNoErr(projectName, taskJobSpec.Env, envMap))
			spec.EnvAlias = commonutil.GetEnvAlias(commonutil.GetEnvInfoNoErr(projectName, taskJobSpec.Env, envMap))
			spec.ServiceType = taskJobSpec.ServiceType
			spec.DeployContents = taskJobSpec.DeployContents
			spec.VariableKVs = taskJobSpec.VariableKVs
			spec.YamlContent = taskJobSpec.YamlContent
			spec.SkipCheckRunStatus = taskJobSpec.SkipCheckRunStatus
			spec.OriginRevision = taskJobSpec.OriginRevision
			// for compatibility
			if taskJobSpec.ServiceModule != "" {
				spec.ServiceAndImages = append(spec.ServiceAndImages, &ServiceAndImage{
					ServiceName:   taskJobSpec.ServiceName,
					ServiceModule: taskJobSpec.ServiceModule,
					Image:         taskJobSpec.Image,
				})
			}

			for _, imageAndmodule := range taskJobSpec.ServiceAndImages {
				spec.ServiceAndImages = append(spec.ServiceAndImages, &ServiceAndImage{
					ServiceName:   taskJobSpec.ServiceName,
					ServiceModule: imageAndmodule.ServiceModule,
					Image:         imageAndmodule.Image,
				})
			}
			jobPreview.Spec = spec
		case string(config.JobZadigHelmDeploy):
			jobPreview.JobType = string(config.JobZadigDeploy)
			spec := ZadigDeployJobPreviewSpec{}
			job.JobType = string(config.JobZadigDeploy)
			taskJobSpec := &commonmodels.JobTaskHelmDeploySpec{}
			if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
				continue
			}
			spec.Env = taskJobSpec.Env
			spec.Production = commonutil.GetEnvProduction(commonutil.GetEnvInfoNoErr(projectName, taskJobSpec.Env, envMap))
			spec.EnvAlias = commonutil.GetEnvAlias(commonutil.GetEnvInfoNoErr(projectName, taskJobSpec.Env, envMap))
			spec.ServiceType = taskJobSpec.ServiceType
			spec.DeployContents = taskJobSpec.DeployContents
			spec.YamlContent = taskJobSpec.YamlContent
			spec.UserSuppliedValue = taskJobSpec.UserSuppliedValue
			spec.SkipCheckRunStatus = taskJobSpec.SkipCheckRunStatus
			spec.ValueMergeStrategy = taskJobSpec.ValueMergeStrategy
			spec.OriginRevision = taskJobSpec.OriginRevision
			for _, imageAndmodule := range taskJobSpec.ImageAndModules {
				spec.ServiceAndImages = append(spec.ServiceAndImages, &ServiceAndImage{
					ServiceName:   taskJobSpec.ServiceName,
					ServiceModule: imageAndmodule.ServiceModule,
					Image:         imageAndmodule.Image,
				})
			}
			jobPreview.Spec = spec
		case string(config.JobZadigHelmChartDeploy):
			jobPreview.JobType = string(config.JobZadigHelmChartDeploy)
			spec := commonmodels.ZadigHelmChartDeployJobSpec{}
			job.JobType = string(config.JobZadigHelmChartDeploy)
			taskJobSpec := &commonmodels.JobTaskHelmChartDeploySpec{}
			if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
				continue
			}
			spec.Env = taskJobSpec.Env
			spec.Production = commonutil.GetEnvProduction(commonutil.GetEnvInfoNoErr(projectName, taskJobSpec.Env, envMap))
			spec.EnvAlias = commonutil.GetEnvAlias(commonutil.GetEnvInfoNoErr(projectName, taskJobSpec.Env, envMap))
			spec.SkipCheckRunStatus = taskJobSpec.SkipCheckRunStatus
			spec.DeployHelmCharts = append(spec.DeployHelmCharts, taskJobSpec.DeployHelmChart)
			jobPreview.Spec = spec
		case string(config.JobZadigVMDeploy):
			spec := commonmodels.ZadigVMDeployJobSpec{}
			taskJobSpec := &commonmodels.JobTaskFreestyleSpec{}
			if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
				continue
			}

			serviceModule := ""
			serviceName := ""
			image := ""
			deployArtifactType := ""
			for _, arg := range taskJobSpec.Properties.Envs {
				if arg.Key == "ENV_NAME" {
					spec.Env = arg.Value
					continue
				}
				if arg.Key == "SERVICE_MODULE" {
					serviceModule = arg.Value
				}
				if arg.Key == "SERVICE_NAME" {
					serviceName = arg.Value
				}
				if arg.Key == "IMAGE" {
					image = arg.Value
				}
				if arg.Key == "DEPLOY_ARTIFACT_TYPE" {
					deployArtifactType = arg.Value
				}
			}

			spec.Production = commonutil.GetEnvProduction(commonutil.GetEnvInfoNoErr(projectName, spec.Env, envMap))
			spec.EnvAlias = commonutil.GetEnvAlias(commonutil.GetEnvInfoNoErr(projectName, spec.Env, envMap))

			serviceAndVMDeploy := &commonmodels.ServiceAndVMDeploy{
				ServiceName:        serviceName,
				ServiceModule:      serviceModule,
				Image:              image,
				DeployArtifactType: types.VMDeployArtifactType(deployArtifactType),
				Repos:              []*types.Repository{},
			}

			for _, step := range taskJobSpec.Steps {
				if step.StepType == config.StepGit {
					stepSpec := &stepspec.StepGitSpec{}
					commonmodels.IToi(step.Spec, &stepSpec)
					serviceAndVMDeploy.Repos = append(serviceAndVMDeploy.Repos, stepSpec.Repos...)
					continue
				} else if step.StepType == config.StepPerforce {
					stepSpec := &stepspec.StepP4Spec{}
					commonmodels.IToi(step.Spec, &stepSpec)
					serviceAndVMDeploy.Repos = append(serviceAndVMDeploy.Repos, stepSpec.Repos...)
					continue
				}

				if step.StepType == config.StepDownloadArchive {
					stepSpec := &stepspec.StepDownloadArchiveSpec{}
					if err := commonmodels.IToi(step.Spec, &stepSpec); err != nil {
						continue
					}

					url := path.Join(stepSpec.S3.Endpoint, stepSpec.S3.Bucket)
					if len(stepSpec.S3.Subfolder) > 0 {
						url = path.Join(url, strings.TrimLeft(stepSpec.S3.Subfolder, "/"))
					}
					url = path.Join(url, stepSpec.ObjectPath, stepSpec.FileName)
					serviceAndVMDeploy.ArtifactURL = url
				}
			}

			serviceAndVMDeploys := []*commonmodels.ServiceAndVMDeploy{
				serviceAndVMDeploy,
			}

			spec.ServiceAndVMDeploys = serviceAndVMDeploys
			jobPreview.Spec = spec
		case string(config.JobPlugin):
			taskJobSpec := &commonmodels.JobTaskPluginSpec{}
			if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
				continue
			}
			jobPreview.Spec = taskJobSpec.Plugin
		case string(config.JobCustomDeploy):
			spec := CustomDeployJobSpec{}
			taskJobSpec := &commonmodels.JobTaskCustomDeploySpec{}
			if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
				continue
			}
			spec.Image = taskJobSpec.Image
			spec.Namespace = taskJobSpec.Namespace
			spec.SkipCheckRunStatus = taskJobSpec.SkipCheckRunStatus
			spec.Target = strings.Join([]string{taskJobSpec.WorkloadType, taskJobSpec.WorkloadName, taskJobSpec.ContainerName}, "/")
			cluster, err := commonrepo.NewK8SClusterColl().Get(taskJobSpec.ClusterID)
			if err != nil {
				log.Errorf("cluster id: %s not found", taskJobSpec.ClusterID)
			} else {
				spec.ClusterName = cluster.Name
			}
			jobPreview.Spec = spec
		case string(config.JobK8sCanaryDeploy):
			taskJobSpec := &commonmodels.JobTaskCanaryDeploySpec{}
			if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
				continue
			}
			sepc := K8sCanaryDeployJobSpec{
				Image:          taskJobSpec.Image,
				K8sServiceName: taskJobSpec.K8sServiceName,
				Namespace:      taskJobSpec.Namespace,
				ContainerName:  taskJobSpec.ContainerName,
				CanaryReplica:  taskJobSpec.CanaryReplica,
				Events:         taskJobSpec.Events,
			}
			cluster, err := commonrepo.NewK8SClusterColl().Get(taskJobSpec.ClusterID)
			if err != nil {
				log.Errorf("cluster id: %s not found", taskJobSpec.ClusterID)
			} else {
				sepc.ClusterName = cluster.Name
			}
			jobPreview.Spec = sepc
		case string(config.JobK8sCanaryRelease):
			taskJobSpec := &commonmodels.JobTaskCanaryReleaseSpec{}
			if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
				continue
			}
			sepc := K8sCanaryReleaseJobSpec{
				Image:          taskJobSpec.Image,
				K8sServiceName: taskJobSpec.K8sServiceName,
				Namespace:      taskJobSpec.Namespace,
				ContainerName:  taskJobSpec.ContainerName,
				Events:         taskJobSpec.Events,
			}
			cluster, err := commonrepo.NewK8SClusterColl().Get(taskJobSpec.ClusterID)
			if err != nil {
				log.Errorf("cluster id: %s not found", taskJobSpec.ClusterID)
			} else {
				sepc.ClusterName = cluster.Name
			}
			jobPreview.Spec = sepc
		default:
			jobPreview.Spec = job.Spec
		}
		resp = append(resp, jobPreview)
	}
	return resp
}

func workflowTaskLint(workflowTask *commonmodels.WorkflowTask, logger *zap.SugaredLogger) error {
	if len(workflowTask.Stages) <= 0 {
		errMsg := fmt.Sprintf("no stage found in workflow task: %s,taskID: %d", workflowTask.WorkflowName, workflowTask.TaskID)
		logger.Error(errMsg)
		return e.ErrCreateTask.AddDesc(errMsg)
	}
	for _, stage := range workflowTask.Stages {
		if len(stage.Jobs) <= 0 {
			errMsg := fmt.Sprintf("no job found in workflow task: %s,taskID: %d,stage: %s", workflowTask.WorkflowName, workflowTask.TaskID, stage.Name)
			logger.Error(errMsg)
			return e.ErrCreateTask.AddDesc(errMsg)
		}

		for _, job := range stage.Jobs {
			if job.JobType == string(config.JobApproval) {
				spec := &commonmodels.JobTaskApprovalSpec{}
				err := commonmodels.IToi(job.Spec, spec)
				if err != nil {
					logger.Errorf("failed to update approval job user info, error: %s", err)
					return e.ErrCreateTask.AddDesc(fmt.Sprintf("failed to update approval job user info, error: %s", err))
				}

				if spec.Type == config.NativeApproval && spec.NativeApproval != nil && len(spec.NativeApproval.ApproveUsers) != 0 {
					newApproveUserList := make([]*commonmodels.User, 0)
					userSet := sets.NewString()
					for _, approveUser := range spec.NativeApproval.ApproveUsers {
						if approveUser.Type == "" || approveUser.Type == "user" {
							newApproveUserList = append(newApproveUserList, approveUser)
							userSet.Insert(approveUser.UserID)
						}
					}
					for _, approveUser := range spec.NativeApproval.ApproveUsers {
						if approveUser.Type == "group" {
							users, err := user.New().GetGroupDetailedInfo(approveUser.GroupID)
							if err != nil {
								errMsg := fmt.Sprintf("failed to find users for group %s in stage: %s, error: %s", approveUser.GroupName, stage.Name, err)
								logger.Errorf(errMsg)
								return e.ErrCreateTask.AddDesc(errMsg)
							}
							for _, userID := range users.UIDs {
								if userSet.Has(userID) {
									continue
								}
								userDetailedInfo, err := user.New().GetUserByID(userID)
								if err != nil {
									errMsg := fmt.Sprintf("failed to find user %s, error: %s", userID, err)
									logger.Errorf(errMsg)
									return e.ErrCreateTask.AddDesc(errMsg)
								}

								userSet.Insert(userID)
								newApproveUserList = append(newApproveUserList, &commonmodels.User{
									Type:     "user",
									UserID:   userID,
									UserName: userDetailedInfo.Name,
								})
							}
						}
					}
					spec.NativeApproval.ApproveUsers = newApproveUserList
				}

				job.Spec = spec
			}
		}
	}
	return nil
}

func GetWorkflowV4ArtifactFileContent(workflowName, jobName string, taskID int64, log *zap.SugaredLogger) ([]byte, error) {
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		return []byte{}, fmt.Errorf("cannot find workflow task, workflow name: %s, task id: %d", workflowName, taskID)
	}
	var jobTask *commonmodels.JobTask
	for _, stage := range workflowTask.Stages {
		for _, job := range stage.Jobs {
			if job.Name != jobName {
				continue
			}
			if job.JobType != string(config.JobZadigTesting) && job.JobType != string(config.JobZadigScanning) {
				return []byte{}, fmt.Errorf("job: %s was not a testing or scanning job", jobName)
			}

			jobTask = job
		}
	}
	if jobTask == nil {
		return []byte{}, fmt.Errorf("cannot find job task, workflow name: %s, task id: %d, job name: %s", workflowName, taskID, jobName)
	}
	jobSpec := &commonmodels.JobTaskFreestyleSpec{}
	if err := commonmodels.IToi(jobTask.Spec, jobSpec); err != nil {
		return []byte{}, fmt.Errorf("unmashal job spec error: %v", err)
	}

	var stepTask *commonmodels.StepTask
	for _, step := range jobSpec.Steps {
		if step.Name != config.TestJobArchiveResultStepName {
			continue
		}
		if step.StepType != config.StepTarArchive {
			return []byte{}, fmt.Errorf("step: %s was not a junit report step", step.Name)
		}
		stepTask = step
	}
	if stepTask == nil {
		return []byte{}, fmt.Errorf("cannot find step task, workflow name: %s, task id: %d, job name: %s", workflowName, taskID, jobName)
	}
	stepSpec := &step.StepTarArchiveSpec{}
	if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil {
		return []byte{}, fmt.Errorf("unmashal step spec error: %v", err)
	}

	storage, err := s3.FindDefaultS3()
	if err != nil {
		log.Errorf("GetTestArtifactInfo FindDefaultS3 err:%v", err)
		return []byte{}, fmt.Errorf("findDefaultS3 err: %v", err)
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Region, storage.Insecure, storage.Provider)
	if err != nil {
		log.Errorf("GetTestArtifactInfo Create S3 client err:%+v", err)
		return []byte{}, fmt.Errorf("create S3 client err: %v", err)
	}
	objectKey := filepath.Join(stepSpec.S3DestDir, stepSpec.FileName)
	object, err := client.GetFile(storage.Bucket, objectKey, &s3tool.DownloadOption{RetryNum: 2})
	if err != nil {
		log.Errorf("GetTestArtifactInfo GetFile err:%s", err)
		return []byte{}, fmt.Errorf("GetFile err: %v", err)
	}
	fileByts, err := ioutil.ReadAll(object.Body)
	if err != nil {
		log.Errorf("GetTestArtifactInfo ioutil.ReadAll err:%s", err)
		return []byte{}, fmt.Errorf("ioutil.ReadAll err: %v", err)
	}
	return fileByts, nil
}

func GetWorkflowV4BuildJobArtifactFile(workflowName, jobName string, taskID int64, log *zap.SugaredLogger) ([]byte, string, error) {
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		return []byte{}, "", fmt.Errorf("cannot find workflow task, workflow name: %s, task id: %d", workflowName, taskID)
	}
	var jobTask *commonmodels.JobTask
	for _, stage := range workflowTask.Stages {
		for _, job := range stage.Jobs {
			if job.Name != jobName {
				continue
			}
			if job.JobType != string(config.JobZadigBuild) {
				return []byte{}, "", fmt.Errorf("job: %s was not a build job", jobName)
			}

			jobTask = job
		}
	}
	if jobTask == nil {
		return []byte{}, "", fmt.Errorf("cannot find job task, workflow name: %s, task id: %d, job name: %s", workflowName, taskID, jobName)
	}
	jobSpec := &commonmodels.JobTaskFreestyleSpec{}
	if err := commonmodels.IToi(jobTask.Spec, jobSpec); err != nil {
		return []byte{}, "", fmt.Errorf("unmashal job spec error: %v", err)
	}

	var stepTask *commonmodels.StepTask
	for _, step := range jobSpec.Steps {
		if !strings.HasSuffix(step.Name, "-pkgfile-archive") {
			continue
		}
		if step.StepType != config.StepArchive {
			return []byte{}, "", fmt.Errorf("step: %s was not a archive step", step.Name)
		}
		stepTask = step
	}
	if stepTask == nil {
		return []byte{}, "", fmt.Errorf("cannot find step task, workflow name: %s, task id: %d, job name: %s", workflowName, taskID, jobName)
	}
	stepSpec := &step.StepArchiveSpec{}
	if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil {
		return []byte{}, "", fmt.Errorf("unmashal step spec error: %v", err)
	}
	if len(stepSpec.UploadDetail) == 0 {
		return []byte{}, "", fmt.Errorf("step: %s has no upload detail", stepTask.Name)
	}

	storage, err := s3.FindDefaultS3()
	if err != nil {
		log.Errorf("GetWorkflowV4BuildJobArtifactFile FindDefaultS3 err:%v", err)
		return []byte{}, "", fmt.Errorf("findDefaultS3 err: %v", err)
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Region, storage.Insecure, storage.Provider)
	if err != nil {
		log.Errorf("GetWorkflowV4BuildJobArtifactFile Create S3 client err:%+v", err)
		return []byte{}, "", fmt.Errorf("create S3 client err: %v", err)
	}

	stepSpec.UploadDetail[0].DestinationPath = strings.TrimLeft(path.Join(stepSpec.S3.Subfolder, stepSpec.UploadDetail[0].DestinationPath), "/")
	objectKey := filepath.Join(stepSpec.UploadDetail[0].DestinationPath, stepSpec.UploadDetail[0].Name)
	object, err := client.GetFile(storage.Bucket, objectKey, &s3tool.DownloadOption{RetryNum: 2})
	if err != nil {
		log.Errorf("GetWorkflowV4BuildJobArtifactFile GetFile err:%s", err)
		return []byte{}, "", fmt.Errorf("GetFile err: %v", err)
	}
	fileByts, err := ioutil.ReadAll(object.Body)
	if err != nil {
		log.Errorf("GetWorkflowV4BuildJobArtifactFile ioutil.ReadAll err:%s", err)
		return []byte{}, "", fmt.Errorf("ioutil.ReadAll err: %v", err)
	}
	return fileByts, stepSpec.UploadDetail[0].Name, nil
}

func UpdateWorkflowV4TaskRemark(workflowName string, taskID int64, remark string, log *zap.SugaredLogger) error {
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		return fmt.Errorf("cannot find workflow task, workflow name: %s, task id: %d", workflowName, taskID)
	}

	workflowTask.Remark = remark
	return commonrepo.NewworkflowTaskv4Coll().Update(workflowTask.ID.Hex(), workflowTask)
}

type ListWorkflowFilterInfoResponse struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

func ListWorkflowFilterInfo(project, workflow, typeName string, jobName string, logger *zap.SugaredLogger) ([]*ListWorkflowFilterInfoResponse, error) {
	if project == "" || workflow == "" || typeName == "" {
		return []*ListWorkflowFilterInfoResponse{}, fmt.Errorf("paramerter is empty")
	}

	envMap := make(map[string]*commonmodels.Product)

	switch typeName {
	case "creator":
		creators, err := commonrepo.NewworkflowTaskv4Coll().ListCreator(project, workflow)
		if err != nil {
			logger.Errorf("ListWorkflowTaskCreator ListCreator err:%v", err)
			return []*ListWorkflowFilterInfoResponse{}, fmt.Errorf("ListCreator err: %v", err)
		}

		resp := make([]*ListWorkflowFilterInfoResponse, 0)
		for _, creator := range creators {
			resp = append(resp, &ListWorkflowFilterInfoResponse{
				Key:  creator,
				Name: creator,
			})
		}
		return resp, nil
	case "envName":
		workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflow)
		if err != nil {
			logger.Errorf("failed to find workflow %s: %v", workflow, err)
			return []*ListWorkflowFilterInfoResponse{}, fmt.Errorf("failed to find workflow %s: %v", workflow, err)
		}

		resp := make([]*ListWorkflowFilterInfoResponse, 0)
		for _, stage := range workflow.Stages {
			for _, job := range stage.Jobs {
				if job.Name == jobName && job.JobType == config.JobZadigDeploy {
					deploy := &commonmodels.ZadigDeployJobSpec{}
					if err := commonmodels.IToi(job.Spec, deploy); err != nil {
						return nil, err
					}

					env, _ := CheckFixedMarkReturnNoFixedEnv(deploy.Env)
					if envMap[env] == nil {
						envInfo, err := commonutil.GetEnvInfo(project, env, envMap)
						if err != nil {
							return nil, err
						}

						resp = append(resp, &ListWorkflowFilterInfoResponse{
							Key:  envInfo.EnvName,
							Name: envInfo.Alias,
						})
					}
					return resp, nil
				}
			}
		}
		return resp, nil
	case "serviceName":
		services := make([]string, 0)
		serviceList, _ := commonrepo.NewServiceColl().ListMaxRevisions(&commonrepo.ServiceListOption{ProductName: project})
		for _, service := range serviceList {
			for _, container := range service.Containers {
				if !utils.Contains(services, container.Name) {
					services = append(services, container.Name)
				}
			}
		}

		resp := make([]*ListWorkflowFilterInfoResponse, 0)
		for _, service := range services {
			resp = append(resp, &ListWorkflowFilterInfoResponse{
				Key:  service,
				Name: service,
			})
		}
		return resp, nil
	default:
		return nil, fmt.Errorf("queryType parameter is invalid")
	}
}
