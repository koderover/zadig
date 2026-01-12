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

package workflow

import (
	"errors"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmongodb "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	codehostmodels "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/tapd"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

type Workflow struct {
	Name                 string                     `json:"name"`
	DisplayName          string                     `json:"display_name"`
	ProjectName          string                     `json:"projectName"`
	Disabled             bool                       `json:"disabled"`
	UpdateTime           int64                      `json:"updateTime"`
	CreateTime           int64                      `json:"createTime"`
	UpdateBy             string                     `json:"updateBy,omitempty"`
	Schedules            *commonmodels.ScheduleCtrl `json:"schedules,omitempty"`
	SchedulerEnabled     bool                       `json:"schedulerEnabled"`
	EnabledStages        []string                   `json:"enabledStages"`
	IsFavorite           bool                       `json:"isFavorite"`
	WorkflowType         string                     `json:"workflow_type"`
	RecentTask           *TaskInfo                  `json:"recentTask"`
	RecentTasks          []*TaskInfo                `json:"recentTasks"`
	RecentSuccessfulTask *TaskInfo                  `json:"recentSuccessfulTask"`
	RecentFailedTask     *TaskInfo                  `json:"recentFailedTask"`
	AverageExecutionTime float64                    `json:"averageExecutionTime"`
	SuccessRate          float64                    `json:"successRate"`
	Description          string                     `json:"description,omitempty"`
	BaseName             string                     `json:"base_name"`
	BaseRefs             []string                   `json:"base_refs"`
	NeverRun             bool                       `json:"never_run"`
	EnableApprovalTicket bool                       `json:"enable_approval_ticket"`
}

type TaskInfo struct {
	TaskID       int64  `json:"taskID"`
	PipelineName string `json:"pipelineName,omitempty"`
	Status       string `json:"status"`
	TaskCreator  string `json:"task_creator"`
	CreateTime   int64  `json:"create_time"`
	RunningTime  int64  `json:"running_time,omitempty"`
	StartTime    int64  `json:"start_time,omitempty"`
	EndTime      int64  `json:"end_time,omitempty"`
}

type EnvStatus struct {
	EnvName    string `json:"env_name,omitempty"`
	Status     string `json:"status"`
	ErrMessage string `json:"err_message"`
}

type WorkflowCopyItem struct {
	ProjectName    string `json:"project_name"`
	Old            string `json:"old"`
	New            string `json:"new"`
	NewDisplayName string `json:"new_display_name"`
	BaseName       string `json:"base_name"`
}

type BulkCopyWorkflowArgs struct {
	Items []WorkflowCopyItem `json:"items"`
}

type workflowCreateArg struct {
	name              string
	envName           string
	buildStageEnabled bool
	dockerRegistryID  string
}

type workflowCreateArgs struct {
	productName string
	argsMap     map[string]*workflowCreateArg
}

func (args *workflowCreateArgs) addWorkflowArg(envName, dockerRegistryID string, buildStageEnabled bool) {
	wName := fmt.Sprintf("%s-workflow-%s", args.productName, envName)
	// The hosting env workflow name is not bound to the environment
	if envName == "" {
		wName = fmt.Sprintf("%s-workflow", args.productName)
	}
	if !buildStageEnabled {
		wName = fmt.Sprintf("%s-%s-workflow", args.productName, "ops")
	}
	args.argsMap[wName] = &workflowCreateArg{
		name:              wName,
		envName:           envName,
		buildStageEnabled: buildStageEnabled,
		dockerRegistryID:  dockerRegistryID,
	}
}

func (args *workflowCreateArgs) initDefaultWorkflows() {
	args.addWorkflowArg("dev", "", true)
	args.addWorkflowArg("qa", "", true)
	args.addWorkflowArg("", "", false)
}

func (args *workflowCreateArgs) clear() {
	args.argsMap = make(map[string]*workflowCreateArg)
}

type OpenAPICreateCustomWorkflowTaskArgs struct {
	WorkflowName string                         `json:"workflow_key"`
	ProjectName  string                         `json:"project_key"`
	Params       []*CreateCustomTaskParam       `json:"parameters"`
	Inputs       []*CreateCustomTaskJobInput    `json:"inputs"`
	NotifyInputs []*CreateCustomTaskNotifyInput `json:"notify_inputs"`
}

type CreateCustomTaskNotifyInput struct {
	// 工作流配置中第几个通知，从 0 开始
	ID int `json:"id"`
	// 通知类型，支持：feishu 飞书群组通知（自定义机器人）、feishu_app 飞书群组通知（自建应用）、feishu_person 飞书成员通知、dingding 钉钉，wechat 企业微信、msteams Teams、mail 邮件
	Type setting.NotifyWebHookType `json:"type"`
	// 飞书群组通知（自定义机器人）配置
	LarkHookNotificationConfig *CreateCustomTaskLarkHookNotificationConfig `json:"lark_hook_notification_config"`
	// 飞书群通知（自建应用）配置
	LarkGroupNotificationConfig *CreateCustomTaskLarkGroupNotificationConfig `json:"lark_group_notification_config"`
	// 飞书成员通知配置
	LarkPersonNotificationConfig *CreateCustomTaskLarkPersonNotificationConfig `json:"lark_person_notification_config"`
	// 钉钉通知配置
	DingDingNotificationConfig *CreateCustomTaskDingDingNotificationConfig `json:"dingding_notification_config"`
	// 企业微信通知配置
	WechatNotificationConfig *CreateCustomTaskWechatNotificationConfig `json:"wechat_notification_config"`
	// MSTeams通知配置
	MSTeamsNotificationConfig *CreateCustomTaskMSTeamsNotificationConfig `json:"msteams_notification_config"`
	// 邮件通知配置
	MailNotificationConfig *CreateCustomTaskMailNotificationConfig `json:"mail_notification_config"`
}

type CreateCustomTaskLarkUserInfo struct {
	ID string `json:"id"`
	// 支持 open_id、user_id
	IDType string `json:"id_type"`
}

type CreateCustomTaskLarkGroupNotificationConfig struct {
	ChatID  string                         `json:"chat_id"`
	AtUsers []CreateCustomTaskLarkUserInfo `json:"at_users"`
}

type CreateCustomTaskLarkPersonNotificationConfig struct {
	Users []CreateCustomTaskLarkUserInfo `json:"users"`
}

type CreateCustomTaskLarkHookNotificationConfig struct {
	AtUsers []string `json:"at_users"`
	IsAtAll bool     `json:"is_at_all"`
}

type CreateCustomTaskWechatNotificationConfig struct {
	AtUsers []string `json:"at_users"`
	IsAtAll bool     `json:"is_at_all"`
}

type CreateCustomTaskDingDingNotificationConfig struct {
	AtMobiles []string `json:"at_mobiles"`
	IsAtAll   bool     `json:"is_at_all"`
}

type CreateCustomTaskMSTeamsNotificationConfig struct {
	AtEmails []string `json:"at_emails"`
}

type CreateCustomTaskMailNotificationConfig struct {
	UserIDs []string `json:"user_ids"`
}

type CreateCustomTaskParam struct {
	Name       string                   `bson:"name"               json:"name"                  yaml:"name"`
	ParamsType config.WorkflowParamType `bson:"type"               json:"type"                  yaml:"type"`
	Value      string                   `bson:"value"              json:"value"                 yaml:"value,omitempty"`
	Repo       *CreateCustomTaskRepoArg `bson:"repo"               json:"repo"                  yaml:"repo,omitempty"`
}

type CreateCustomTaskRepoArg struct {
	CodeHostName  string `bson:"codehost_name"      json:"codehost_name"        yaml:"codehost_name"`
	RepoNamespace string `bson:"repo_namespace"     json:"repo_namespace"       yaml:"repo_namespace"`
	RepoName      string `bson:"repo_name"          json:"repo_name"            yaml:"repo_name"`
	Branch        string `bson:"branch"             json:"branch"               yaml:"branch"`
	PRs           []int  `bson:"prs"                json:"prs"                  yaml:"prs"`
}

type CreateCustomTaskJobInput struct {
	JobName    string         `json:"job_name"`
	JobType    config.JobType `json:"job_type"`
	Parameters interface{}    `json:"parameters"`
}

type OpenAPICreateProductWorkflowTaskArgs struct {
	WorkflowName string                     `json:"workflow_key"`
	ProjectName  string                     `json:"project_key"`
	Input        *CreateProductTaskJobInput `json:"input"`
}

func (c *OpenAPICreateProductWorkflowTaskArgs) Validate() (bool, error) {
	if c.WorkflowName == "" {
		return false, fmt.Errorf("workflowKey cannot be empty")
	}
	if c.ProjectName == "" {
		return false, fmt.Errorf("projectKey cannot be empty")
	}

	if c.Input == nil {
		return false, fmt.Errorf("input cannot be empty")
	}
	return true, nil
}

type OpenAPIBasicInfo struct {
	workflow *commonmodels.WorkflowV4
}

type CreateProductTaskJobInput struct {
	TargetEnv  string            `json:"target_env"`
	BuildArgs  WorkflowBuildArg  `json:"build"`
	DeployArgs WorkflowDeployArg `json:"deploy"`
}

type WorkflowBuildArg struct {
	Enabled     bool                             `json:"enabled"`
	ServiceList []*types.OpenAPIServiceBuildArgs `json:"service_list"`
}

type WorkflowDeployArg struct {
	Enabled     bool                 `json:"enabled"`
	Source      string               `json:"source"`
	ServiceList []*ServiceDeployArgs `json:"service_list"`
}

type CustomJobInput interface {
	UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error)
}

type PluginJobInput struct {
	KVs []*types.KV `json:"kv"`
}

func (p *PluginJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.PluginJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.PluginJobSpec")
	}
	kvMap := make(map[string]string)
	for _, kv := range p.KVs {
		kvMap[kv.Key] = kv.Value
	}

	for _, param := range newSpec.Plugin.Inputs {
		if val, ok := kvMap[param.Name]; ok {
			param.Value = val
		}
	}

	job.Spec = newSpec

	return job, nil
}

type FreestyleJobServiceInfo struct {
	ServiceName   string                    `json:"service_name"`
	ServiceModule string                    `json:"service_module"`
	RepoInfo      []*types.OpenAPIRepoInput `json:"repo_info"`
	Inputs        []*types.KV               `json:"inputs"`
}

type FreestyleJobInput struct {
	*OpenAPIBasicInfo

	FreestyleJobType config.FreeStyleJobType `json:"freestyle_type"`

	// for service freestyle job
	Services []*FreestyleJobServiceInfo `json:"services"`

	// for non-service freestyle job
	KVs      []*types.KV               `json:"kv"`
	RepoInfo []*types.OpenAPIRepoInput `json:"repo_info"`
}

func OpenAPIKVInputToKeyValList(originalKvs commonmodels.RuntimeKeyValList, kvInputs []*types.KV) commonmodels.RuntimeKeyValList {
	newKVs := make(commonmodels.RuntimeKeyValList, len(originalKvs))
	for i, kv := range originalKvs {
		util.DeepCopy(&newKVs[i], kv)
	}

	for _, kv := range newKVs {
		kvMap := make(map[string]*types.KV, len(kvInputs))
		for _, kv := range kvInputs {
			kvMap[kv.Key] = kv
		}

		if kvInput, ok := kvMap[kv.Key]; ok {
			kv.Value = kvInput.Value
		}
	}

	return newKVs
}

// getCodeHostInfoMap 批量获取并缓存 CodeHost 信息
func getCodeHostInfoMap(repoInputs []*types.OpenAPIRepoInput) (map[string]*codehostmodels.CodeHost, error) {
	repoInfoMap := make(map[string]*codehostmodels.CodeHost)
	for _, inputRepo := range repoInputs {
		if _, exists := repoInfoMap[inputRepo.CodeHostName]; exists {
			continue
		}
		repoInfo, err := mongodb.NewCodehostColl().GetSystemCodeHostByAlias(inputRepo.CodeHostName)
		if err != nil {
			return nil, errors.New("failed to find code host with name:" + inputRepo.CodeHostName)
		}
		repoInfoMap[inputRepo.CodeHostName] = repoInfo
	}
	return repoInfoMap, nil
}

func OpenAPIRepoInputToRepository(originalRepos []*types.Repository, repoInpus []*types.OpenAPIRepoInput) ([]*types.Repository, error) {
	if len(repoInpus) == 0 {
		return originalRepos, nil
	}

	// 批量获取并缓存 CodeHost 信息，避免重复查询
	repoInfoMap, err := getCodeHostInfoMap(repoInpus)
	if err != nil {
		return nil, err
	}

	newRepo := make([]*types.Repository, 0)

	for _, repo := range originalRepos {
		for _, inputRepo := range repoInpus {
			repoInfo := repoInfoMap[inputRepo.CodeHostName]
			if repo.CodehostID != repoInfo.ID {
				continue
			}

			if repoInfo.Type != "perforce" {
				if repo.RepoNamespace == inputRepo.RepoNamespace && repo.RepoName == inputRepo.RepoName {
					repo.Branch = inputRepo.Branch
					repo.PR = inputRepo.PR
					repo.PRs = inputRepo.PRs
					repo.EnableCommit = inputRepo.EnableCommit
					repo.CommitID = inputRepo.CommitID
					newRepo = append(newRepo, repo)
				}
			} else {
				var depotType string
				if inputRepo.Stream != "" {
					depotType = "stream"
				} else {
					depotType = "local"
				}
				newRepo = append(newRepo, &types.Repository{
					Source:       repoInfo.Type,
					CodehostID:   repoInfo.ID,
					Username:     repoInfo.Username,
					Password:     repoInfo.Password,
					PerforceHost: repoInfo.P4Host,
					PerforcePort: repoInfo.P4Port,
					DepotType:    depotType,
					Stream:       inputRepo.Stream,
					ViewMapping:  inputRepo.ViewMapping,
					ChangeListID: inputRepo.ChangelistID,
					ShelveID:     inputRepo.ShelveID,
				})
			}
		}
	}

	return newRepo, nil
}

func (p *FreestyleJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.FreestyleJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.FreestyleJobSpec")
	}

	if newSpec.FreestyleJobType == config.ServiceFreeStyleJobType {
		if newSpec.ServiceSource == config.SourceFromJob {
			err := p.getReferredJobTargets(newSpec)
			if err != nil {
				return nil, err
			}
		} else {
			services := make([]*commonmodels.FreeStyleServiceInfo, 0)
			for _, service := range p.Services {
				newRepos, err := OpenAPIRepoInputToRepository(newSpec.Repos, service.RepoInfo)
				if err != nil {
					return nil, err
				}
				newKvs := OpenAPIKVInputToKeyValList(newSpec.Envs, service.Inputs)
				services = append(services, &commonmodels.FreeStyleServiceInfo{
					ServiceWithModule: commonmodels.ServiceWithModule{
						ServiceName:   service.ServiceName,
						ServiceModule: service.ServiceModule,
					},
					Repos:   newRepos,
					KeyVals: newKvs,
				})
			}
			newSpec.Services = services
		}
	} else if newSpec.FreestyleJobType == config.NormalFreeStyleJobType {
		newSpec.Envs = OpenAPIKVInputToKeyValList(newSpec.Envs, p.KVs)

		newRepos, err := OpenAPIRepoInputToRepository(newSpec.Repos, p.RepoInfo)
		if err != nil {
			return nil, err
		}
		newSpec.Repos = newRepos
	} else {
		return nil, fmt.Errorf("freestyle job type %s not supported", newSpec.FreestyleJobType)
	}

	job.Spec = newSpec

	return job, nil
}

func (p *FreestyleJobInput) getReferredJobTargets(jobSpec *commonmodels.FreestyleJobSpec) error {
	if jobSpec.ServiceSource != config.SourceFromJob || jobSpec.FreestyleJobType != config.ServiceFreeStyleJobType {
		return nil
	}

	serviceInputMap := make(map[string]*FreestyleJobServiceInfo)
	for _, service := range p.Services {
		serviceInputMap[service.ServiceName+"-"+service.ServiceModule] = service
	}

	for _, stage := range p.workflow.Stages {
		for _, referredJob := range stage.Jobs {
			if referredJob.Name != jobSpec.JobName {
				continue
			}
			if referredJob.JobType == config.JobZadigBuild {
				buildSpec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToi(referredJob.Spec, buildSpec); err != nil {
					return err
				}

				serviceTargets := make([]*commonmodels.FreeStyleServiceInfo, 0)
				for _, build := range buildSpec.ServiceAndBuilds {
					target := &commonmodels.FreeStyleServiceInfo{
						ServiceWithModule: commonmodels.ServiceWithModule{
							ServiceName:   build.ServiceName,
							ServiceModule: build.ServiceModule,
						},
					}

					if jobSpec.RefRepos {
						target.Repos = build.Repos
					}

					if _, ok := serviceInputMap[target.GetKey()]; ok {
						if !jobSpec.RefRepos {
							newRepos, err := OpenAPIRepoInputToRepository(jobSpec.Repos, serviceInputMap[target.GetKey()].RepoInfo)
							if err != nil {
								return err
							}
							target.Repos = newRepos
						}

						target.KeyVals = OpenAPIKVInputToKeyValList(jobSpec.Envs, serviceInputMap[target.GetKey()].Inputs)
					}

					serviceTargets = append(serviceTargets, target)
				}

				jobSpec.Services = serviceTargets
				return nil
			}
		}
	}
	return fmt.Errorf("FreeStyleJob: refered job %s not found", jobSpec.JobName)
}

type ZadigBuildJobInput struct {
	Registry    string                           `json:"registry"`
	ServiceList []*types.OpenAPIServiceBuildArgs `json:"service_list"`
}

func (p *ZadigBuildJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.ZadigBuildJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.ZadigBuildJobSpec")
	}

	// first convert registry name into registry id
	registryList, err := commonrepo.NewRegistryNamespaceColl().FindAll(&commonrepo.FindRegOps{})
	if err != nil {
		return nil, errors.New("failed to list registries")
	}

	regID := ""
	for _, registry := range registryList {
		regName := fmt.Sprintf("%s/%s", registry.RegAddr, registry.Namespace)
		if regName == p.Registry {
			regID = registry.ID.Hex()
			break
		}
	}
	if regID == "" {
		return nil, errors.New("didn't find the specified image registry")
	}

	// set the id to the spec
	newSpec.DockerRegistryID = regID

	buildSvcList := make([]*commonmodels.ServiceAndBuild, 0)
	for _, svcBuild := range newSpec.ServiceAndBuildsOptions {
		for _, inputSvc := range p.ServiceList {
			// if the service & service module match, we do the update logic
			if inputSvc.ServiceName == svcBuild.ServiceName && inputSvc.ServiceModule == svcBuild.ServiceModule {
				// update build repo info with input build info
				for _, inputRepo := range inputSvc.RepoInfo {
					repoInfo, err := mongodb.NewCodehostColl().GetSystemCodeHostByAlias(inputRepo.CodeHostName)
					if err != nil {
						return nil, errors.New("failed to find code host with name:" + inputRepo.CodeHostName)
					}

					for _, buildRepo := range svcBuild.Repos {
						if buildRepo.CodehostID == repoInfo.ID {
							if buildRepo.RepoNamespace == inputRepo.RepoNamespace && buildRepo.RepoName == inputRepo.RepoName {
								buildRepo.Branch = inputRepo.Branch
								buildRepo.PR = inputRepo.PR
								buildRepo.PRs = inputRepo.PRs
								buildRepo.EnableCommit = inputRepo.EnableCommit
								buildRepo.CommitID = inputRepo.CommitID
							}
						}
					}
				}

				// update the build kv
				kvMap := make(map[string]string)
				for _, kv := range inputSvc.Inputs {
					kvMap[kv.Key] = kv.Value
				}

				for _, buildParam := range svcBuild.KeyVals {
					if val, ok := kvMap[buildParam.Key]; ok {
						buildParam.Value = val
					}
				}

				buildSvcList = append(buildSvcList, svcBuild)
			}
		}
	}

	newSpec.ServiceAndBuilds = buildSvcList
	job.Spec = newSpec

	return job, nil
}

type ZadigDeployJobInput struct {
	EnvName     string               `json:"env_name"` // required
	ServiceList []*ServiceDeployArgs `json:"service_list"`
}

type ServiceDeployArgs struct {
	ServiceModule      string                    `json:"service_module"`
	ServiceName        string                    `json:"service_name"`
	ImageName          string                    `json:"image_name"`
	ValueMergeStrategy config.ValueMergeStrategy `json:"value_merge_strategy,omitempty"`
}

func (p *ZadigDeployJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.ZadigDeployJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.ZadigDeployJobSpec")
	}

	newSpec.Env = p.EnvName
	newSpec.Services = make([]*commonmodels.DeployServiceInfo, 0)
	serviceMap := map[string]*commonmodels.DeployServiceInfo{}
	for _, inputSvc := range p.ServiceList {
		if service, ok := serviceMap[inputSvc.ServiceName]; ok {
			service.Modules = append(service.Modules, &commonmodels.DeployModuleInfo{
				Image:         inputSvc.ImageName,
				ServiceModule: inputSvc.ServiceModule,
			})
			service.ValueMergeStrategy = inputSvc.ValueMergeStrategy
		} else {
			basicInfo := commonmodels.DeployBasicInfo{
				ServiceName: inputSvc.ServiceName,
				Modules: append([]*commonmodels.DeployModuleInfo{}, &commonmodels.DeployModuleInfo{
					Image:         inputSvc.ImageName,
					ServiceModule: inputSvc.ServiceModule,
				}),
			}
			serviceMap[inputSvc.ServiceName] = &commonmodels.DeployServiceInfo{
				DeployBasicInfo: basicInfo,
				DeployVariableInfo: commonmodels.DeployVariableInfo{
					ValueMergeStrategy: inputSvc.ValueMergeStrategy,
				},
			}
			newSpec.Services = append(newSpec.Services, serviceMap[inputSvc.ServiceName])
		}
	}

	job.Spec = newSpec

	return job, nil
}

type BlueGreenDeployJobInput struct {
	ServiceList []*BlueGreenDeployArgs `json:"service_list"`
}

type BlueGreenDeployArgs struct {
	ServiceName string `json:"service_name"`
	ImageName   string `json:"image_name"`
}

func (p *BlueGreenDeployJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.BlueGreenDeployJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.BlueGreenDeployJobSpec")
	}

	for _, svcDeploy := range newSpec.Targets {
		for _, inputSvc := range p.ServiceList {
			if inputSvc.ServiceName == svcDeploy.K8sServiceName {
				svcDeploy.Image = inputSvc.ImageName
			}
		}
	}

	job.Spec = newSpec

	return job, nil
}

type CanaryDeployJobInput struct {
	ServiceList []*BlueGreenDeployArgs `json:"service_list"`
}

type CanaryDeployArgs struct {
	ServiceName string `json:"service_name"`
	ImageName   string `json:"image_name"`
}

func (p *CanaryDeployJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.CanaryDeployJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.CanaryDeployJobSpec")
	}

	for _, svcDeploy := range newSpec.Targets {
		for _, inputSvc := range p.ServiceList {
			if inputSvc.ServiceName == svcDeploy.K8sServiceName {
				svcDeploy.Image = inputSvc.ImageName
			}
		}
	}

	job.Spec = newSpec

	return job, nil
}

type CustomDeployJobInput struct {
	TargetList []*CustomDeployTarget `json:"target_list"`
}

type CustomDeployTarget struct {
	WorkloadType  string `json:"workload_type"`
	WorkloadName  string `json:"workload_name"`
	ContainerName string `json:"container_name"`
	ImageName     string `json:"image_name"`
}

func (p *CustomDeployJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.CustomDeployJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.CustomDeployJobSpec")
	}

	newTargets := make([]*commonmodels.DeployTargets, 0)

	for _, target := range p.TargetList {
		newTargets = append(newTargets, &commonmodels.DeployTargets{
			Target: fmt.Sprintf("%s/%s/%s", target.WorkloadType, target.WorkloadName, target.ContainerName),
			Image:  target.ImageName,
		})
	}

	newSpec.Targets = newTargets

	job.Spec = newSpec

	return job, nil
}

type EmptyInput struct{}

func (p *EmptyInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	return job, nil
}

type ZadigTestingJobInput struct {
	TestingList []*TestingArgs `json:"testing_list"`
}

type TestingArgs struct {
	TestingName string                    `json:"testing_name"`
	RepoInfo    []*types.OpenAPIRepoInput `json:"repo_info"`
	Inputs      []*types.KV               `json:"inputs"`
}

func (p *ZadigTestingJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.ZadigTestingJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.ZadigTestingJobSpec")
	}

	for _, testing := range newSpec.TestModules {
		for _, inputTesting := range p.TestingList {
			// if the testing name match, we do the update logic
			if inputTesting.TestingName == testing.Name {
				// update build repo info with input build info
				for _, inputRepo := range inputTesting.RepoInfo {
					repoInfo, err := mongodb.NewCodehostColl().GetSystemCodeHostByAlias(inputRepo.CodeHostName)
					if err != nil {
						return nil, errors.New("failed to find code host with name:" + inputRepo.CodeHostName)
					}

					for _, buildRepo := range testing.Repos {
						if buildRepo.CodehostID == repoInfo.ID {
							if buildRepo.RepoNamespace == inputRepo.RepoNamespace && buildRepo.RepoName == inputRepo.RepoName {
								buildRepo.Branch = inputRepo.Branch
								buildRepo.PR = inputRepo.PR
								buildRepo.PRs = inputRepo.PRs
								buildRepo.EnableCommit = inputRepo.EnableCommit
								buildRepo.CommitID = inputRepo.CommitID
							}
						}
					}
				}

				// update the build kv
				kvMap := make(map[string]string)
				for _, kv := range inputTesting.Inputs {
					kvMap[kv.Key] = kv.Value
				}

				for _, buildParam := range testing.KeyVals {
					if val, ok := kvMap[buildParam.Key]; ok {
						buildParam.Value = val
					}
				}
			}
		}
	}

	job.Spec = newSpec

	return job, nil
}

type GrayReleaseJobInput struct {
	TargetList []*GrayReleaseTarget `json:"target_list"`
}

type GrayReleaseTarget struct {
	WorkloadType  string `json:"workload_type"`
	WorkloadName  string `json:"workload_name"`
	ContainerName string `json:"container_name"`
	ImageName     string `json:"image_name"`
}

func (p *GrayReleaseJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.GrayReleaseJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.GrayReleaseJobSpec")
	}

	newTargets := []*commonmodels.GrayReleaseTarget{}

	for _, target := range newSpec.Targets {
		for _, inputTarget := range p.TargetList {
			if target.WorkloadName != inputTarget.WorkloadName {
				continue
			}
			target.Image = inputTarget.ImageName
			newTargets = append(newTargets, target)
		}
	}

	newSpec.Targets = newTargets

	job.Spec = newSpec

	return job, nil
}

type GrayRollbackJobInput struct {
	TargetList []*GrayReleaseTarget `json:"target_list"`
}

type GrayRollbackTarget struct {
	WorkloadType string `json:"workload_type"`
	WorkloadName string `json:"workload_name"`
}

func (p *GrayRollbackJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.GrayRollbackJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.GrayRollbackJobSpec")
	}

	newTargets := []*commonmodels.GrayRollbackTarget{}

	for _, target := range newSpec.Targets {
		for _, inputTarget := range p.TargetList {
			if target.WorkloadName != inputTarget.WorkloadName {
				continue
			}
			newTargets = append(newTargets, target)
		}
	}

	newSpec.Targets = newTargets

	job.Spec = newSpec

	return job, nil
}

type K8sPatchJobInput struct {
	TargetList []*K8sPatchTarget `json:"target_list"`
}

type K8sPatchTarget struct {
	ResourceName    string      `json:"resource_name"`
	ResourceKind    string      `json:"resource_kind"`
	ResourceGroup   string      `json:"resource_group"`
	ResourceVersion string      `json:"resource_version"`
	Inputs          []*types.KV `json:"inputs"`
}

func (p *K8sPatchJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.K8sPatchJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.K8sPatchJobSpec")
	}

	newItems := []*commonmodels.PatchItem{}

	for _, target := range newSpec.PatchItems {
		for _, inputTarget := range p.TargetList {
			if target.ResourceName == inputTarget.ResourceName && target.ResourceGroup == inputTarget.ResourceGroup && target.ResourceVersion == inputTarget.ResourceVersion && target.ResourceKind == inputTarget.ResourceKind {
				// update the render kv
				kvMap := make(map[string]string)
				for _, kv := range inputTarget.Inputs {
					kvMap[kv.Key] = kv.Value
				}

				for _, param := range target.Params {
					if val, ok := kvMap[param.Name]; ok {
						param.Value = val
					}
				}
				newItems = append(newItems, target)
			}
		}
	}

	newSpec.PatchItems = newItems

	job.Spec = newSpec

	return job, nil
}

type ZadigScanningJobInput struct {
	ScanningList []*ScanningArg `json:"scanning_list"`
}

type ScanningArg struct {
	ScanningName string                    `json:"scanning_name"`
	RepoInfo     []*types.OpenAPIRepoInput `json:"repo_info"`
}

func (p *ZadigScanningJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.ZadigScanningJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.ZadigScanningJobSpec")
	}

	for _, scanning := range newSpec.Scannings {
		for _, inputScanning := range p.ScanningList {
			// if the scanning name match, we do the update logic
			if inputScanning.ScanningName == scanning.Name {
				// update build repo info with input build info
				for _, inputRepo := range inputScanning.RepoInfo {
					repoInfo, err := mongodb.NewCodehostColl().GetSystemCodeHostByAlias(inputRepo.CodeHostName)
					if err != nil {
						return nil, errors.New("failed to find code host with name:" + inputRepo.CodeHostName)
					}

					for _, buildRepo := range scanning.Repos {
						if buildRepo.CodehostID == repoInfo.ID {
							if buildRepo.RepoNamespace == inputRepo.RepoNamespace && buildRepo.RepoName == inputRepo.RepoName {
								buildRepo.Branch = inputRepo.Branch
								buildRepo.PR = inputRepo.PR
								buildRepo.PRs = inputRepo.PRs
								buildRepo.EnableCommit = inputRepo.EnableCommit
								buildRepo.CommitID = inputRepo.CommitID
							}
						}
					}
				}
			}
		}
	}

	job.Spec = newSpec

	return job, nil
}

type ZadigVMDeployJobInput struct {
	EnvName     string                 `json:"env_name"` // required
	ServiceList []*VMServiceDeployArgs `json:"service_list"`
}

type VMServiceDeployArgs struct {
	ServiceName string `json:"service_name"`
	FileName    string `json:"file_name"`
}

func (p *ZadigVMDeployJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.ZadigVMDeployJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.ZadigVMDeployJobSpec")
	}

	newSpec.Env = p.EnvName
	newSpec.ServiceAndVMDeploys = make([]*commonmodels.ServiceAndVMDeploy, 0)
	serviceMap := map[string]*commonmodels.ServiceAndVMDeploy{}
	for _, inputSvc := range p.ServiceList {
		if _, ok := serviceMap[inputSvc.ServiceName]; ok {
			// previously added, return error since there are 2 services, which is not allowed in the vm deploy
			return nil, fmt.Errorf("cannot deploy 2 same service")
		}

		// register the service but not using it just in case someone decide to deploy the same service more than 1 time.
		serviceMap[inputSvc.ServiceName] = &commonmodels.ServiceAndVMDeploy{
			ServiceName: inputSvc.ServiceName,
		}

		artifacts, err := commonservice.ListTars(newSpec.S3StorageID, "file", []string{inputSvc.ServiceName})
		if err != nil {
			return nil, fmt.Errorf("failed to validate given file: %s, error: %s", inputSvc.FileName, err)
		}

		found := false
		var taskID int64
		workflowType := ""
		workflowName := ""
		jobTaskName := ""

		for _, artifact := range artifacts {
			if artifact.FileName == inputSvc.FileName {
				taskID = artifact.TaskID
				workflowType = artifact.WorkflowType
				workflowName = artifact.WorkflowName
				jobTaskName = artifact.JobTaskName
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("failed to validate given file: %s, error: artifact not found in zadig artifact list", inputSvc.FileName)
		}

		newSpec.ServiceAndVMDeploys = append(newSpec.ServiceAndVMDeploys, &commonmodels.ServiceAndVMDeploy{
			ServiceName:  inputSvc.ServiceName,
			FileName:     inputSvc.FileName,
			TaskID:       int(taskID),
			WorkflowType: config.PipelineType(workflowType),
			WorkflowName: workflowName,
			JobTaskName:  jobTaskName,
		})
	}

	job.Spec = newSpec

	return job, nil
}

type SQLJobInput struct {
	DatabaseName string `json:"database_name"`
	SQL          string `json:"sql"`
}

func (p *SQLJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.SQLJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, fmt.Errorf("failed to convert job.Spec to commonmodels.SQLJobSpec, err: %w", err)
	}

	if newSpec.Source == string(config.SourceRuntime) {
		dbInstance, err := commonrepo.NewDBInstanceColl().Find(&commonrepo.DBInstanceCollFindOption{
			Name: p.DatabaseName,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to find database instance %s, error: %w", p.DatabaseName, err)
		}

		newSpec.ID = dbInstance.ID.Hex()
		newSpec.Type = dbInstance.Type
	}

	newSpec.SQL = p.SQL

	job.Spec = newSpec

	return job, nil
}

type TapdJobInput struct {
	ProjectID    string                     `json:"project_id"`
	ProjectName  string                     `json:"project_name"`
	Status       config.TapdIterationStatus `json:"status"`
	IterationIDs []string                   `json:"iteration_ids"`
}

type TapdJobIterationInput struct {
	IterationID   string `json:"iteration_id"`
	IterationName string `json:"iteration_name"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
}

func (p *TapdJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.TapdJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, fmt.Errorf("failed to convert job.Spec to commonmodels.SQLJobSpec, err: %w", err)
	}

	spec, err := commonmongodb.NewProjectManagementColl().GetTapdSpec(newSpec.TapdID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tapd info, error: %s", err)
	}

	client, err := tapd.NewClient(spec.TapdAddress, spec.TapdClientID, spec.TapdClientSecret, spec.TapdCompanyID)
	if err != nil {
		return nil, fmt.Errorf("failed to create tapd client, error: %s", err)
	}

	projectInfo, err := client.GetProject(p.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project info: %w", err)
	}

	iterations, err := client.GetIterations(p.ProjectID, p.IterationIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get iterations: %w", err)
	}
	iterationMap := make(map[string]*tapd.Iteration)
	for _, iteration := range iterations {
		iterationMap[iteration.ID] = iteration
	}

	newSpec.ProjectID = p.ProjectID
	newSpec.ProjectName = projectInfo.Name
	newSpec.Status = p.Status
	for _, iterationID := range p.IterationIDs {
		iterationInfo, ok := iterationMap[iterationID]
		if !ok {
			return nil, fmt.Errorf("failed to get iteration %s info", iterationID)
		}

		newSpec.Iterations = append(newSpec.Iterations, &commonmodels.TapdIteration{
			IterationID:   iterationID,
			IterationName: iterationInfo.Name,
			StartDate:     iterationInfo.StartDate,
			EndDate:       iterationInfo.EndDate,
		})
	}

	job.Spec = newSpec

	return job, nil
}

type GetHelmValuesDifferenceResp struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
}

type OpenAPIWorkflowV4ListReq struct {
	ProjectKey string `form:"projectKey"`
	ViewName   string `form:"viewName"`
}

type OpenAPIWorkflowListResp struct {
	Workflows []*WorkflowBrief `json:"workflows"`
}

type WorkflowBrief struct {
	WorkflowName string `json:"workflow_key"`
	DisplayName  string `json:"workflow_name"`
	UpdateBy     string `json:"update_by"`
	UpdateTime   int64  `json:"update_time"`
	Type         string `json:"type"`
}

type OpenAPIWorkflowV4Detail struct {
	Name             string                       `json:"workflow_key"`
	DisplayName      string                       `json:"workflow_name"`
	ProjectName      string                       `json:"project_key"`
	Description      string                       `json:"description"`
	CreatedBy        string                       `json:"created_by"`
	CreateTime       int64                        `json:"create_time"`
	UpdatedBy        string                       `json:"updated_by"`
	UpdateTime       int64                        `json:"update_time"`
	Params           []*commonmodels.Param        `json:"params"`
	Stages           []*OpenAPIStage              `json:"stages"`
	NotifyCtls       []*commonmodels.NotifyCtl    `json:"notify_ctls"`
	ShareStorages    []*commonmodels.ShareStorage `json:"share_storages"`
	ConcurrencyLimit int                          `json:"concurrency_limit"`
}

type Param struct {
	Name        string `bson:"name"             json:"name"             yaml:"name"`
	Description string `bson:"description"      json:"description"      yaml:"description"`
	// support string/text/choice/repo type
	ParamsType   string                 `bson:"type"                      json:"type"                        yaml:"type"`
	Value        string                 `bson:"value"                     json:"value"                       yaml:"value,omitempty"`
	Repo         *types.Repository      `bson:"repo"                     json:"repo"                         yaml:"repo,omitempty"`
	ChoiceOption []string               `bson:"choice_option,omitempty"   json:"choice_option,omitempty"     yaml:"choice_option,omitempty"`
	ChoiceValue  []string               `bson:"choice_value,omitempty"    json:"choice_value,omitempty"      yaml:"choice_value,omitempty"`
	Default      string                 `bson:"default"                   json:"default"                     yaml:"default"`
	IsCredential bool                   `bson:"is_credential"             json:"is_credential"               yaml:"is_credential"`
	Source       config.ParamSourceType `bson:"source,omitempty" json:"source,omitempty" yaml:"source,omitempty"`
}

type OpenAPIStage struct {
	Name     string              `json:"name"`
	Parallel bool                `json:"parallel,omitempty"`
	Jobs     []*commonmodels.Job `json:"jobs,omitempty"`
}

type OpenAPIServiceModule struct {
	ServiceModule string `json:"service_module"`
	ServiceName   string `json:"service_name"`
}

type OpenAPIWorkflowV4TaskListResp struct {
	Total         int64                    `json:"total"`
	WorkflowTasks []*OpenAPIWorkflowV4Task `json:"workflow_tasks"`
}

type OpenAPIWorkflowV4Task struct {
	WorkflowName string          `json:"workflow_key"`
	DisplayName  string          `json:"workflow_name"`
	ProjectName  string          `json:"project_key"`
	TaskID       int64           `json:"task_id"`
	CreateTime   int64           `json:"create_time"`
	TaskCreator  string          `json:"task_creator"`
	StartTime    int64           `json:"start_time"`
	EndTime      int64           `json:"end_time"`
	Stages       []*OpenAPIStage `json:"stages,omitempty"`
	Status       config.Status   `json:"status"`
}

type OpenAPIProductWorkflowTaskBrief struct {
	WorkflowName string        `json:"workflow_key"`
	ProjectName  string        `json:"project_key"`
	TaskID       int64         `json:"task_id"`
	CreateTime   int64         `json:"create_time"`
	TaskCreator  string        `json:"task_creator"`
	StartTime    int64         `json:"start_time"`
	EndTime      int64         `json:"end_time"`
	Status       config.Status `json:"status"`
}

type OpenAPIProductWorkflowTaskDetail struct {
	WorkflowName string        `json:"workflow_key"`
	DisplayName  string        `json:"workflow_name,omitempty"`
	ProjectName  string        `json:"project_key"`
	TaskID       int64         `json:"task_id"`
	CreateTime   int64         `json:"create_time"`
	TaskCreator  string        `json:"task_creator"`
	StartTime    int64         `json:"start_time"`
	EndTime      int64         `json:"end_time"`
	Status       config.Status `json:"status"`
}

type OpenAPIWorkflowTaskStage struct {
	Name      string                    `json:"name"`
	Parallel  bool                      `json:"parallel"`
	Approval  *OpenAPIWorkflowApproval  `json:"approval,omitempty"`
	Jobs      []*OpenAPIWorkflowTaskJob `json:"jobs,omitempty"`
	Status    config.Status             `json:"status"`
	Error     string                    `json:"error"`
	StartTime int64                     `json:"start_time"`
	EndTime   int64                     `json:"end_time"`
}

type OpenAPIWorkflowApproval struct {
	Enabled          bool                `json:"enabled"`
	Type             config.ApprovalType `json:"type"`
	Description      string              `json:"description"`
	NativeApproval   *NativeApproval     `json:"native_approval,omitempty"`
	LarkApproval     *LarkApproval       `json:"lark_approval,omitempty"`
	DingTalkApproval *DingTalkApproval   `json:"dingtalk_approval,omitempty"`
}

type NativeApproval struct {
	Timeout         int64          `bson:"timeout"`
	ApproveUsers    []*ApproveUser `json:"approve_users"`
	NeededApprovers int            `bson:"needed_approvers"`
}

type LarkApproval struct {
	Timeout      int64          `bson:"timeout"`
	ApproveUsers []*ApproveUser `json:"approve_users"`
}

type DingTalkApproval struct {
	Timeout int64 `bson:"timeout"`
}

type ApproveUser struct {
	UserName string `json:"user_name"`
	UserID   string `json:"user_id"`
}

type OpenAPIWorkflowTaskJob struct {
	Name           string                  `json:"name"`
	JobType        config.JobType          `json:"job_type"`
	Skipped        bool                    `json:"skipped"`
	RunPolicy      config.JobRunPolicy     `json:"run_policy"`
	ServiceModules []*OpenAPIServiceModule `json:"service_modules"`
	Status         config.Status           `json:"status"`
	Error          string                  `json:"error"`
	StartTime      int64                   `json:"start_time"`
	EndTime        int64                   `json:"end_time"`
}

type OpenAPIPageParamsFromReq struct {
	ProjectKey string `form:"projectKey"`
	PageNum    int64  `form:"pageNum,default=1"`
	PageSize   int64  `form:"pageSize,default=10"`
}

type OpenAPIWorkflowViewBrief struct {
	Name        string          `json:"name"`
	ProjectName string          `json:"project_key"`
	UpdateTime  int64           `json:"update_time"`
	UpdateBy    string          `json:"update_by"`
	Workflows   []*ViewWorkflow `json:"workflows"`
}

type ViewWorkflow struct {
	WorkflowName string `json:"workflow_key"`
	WorkflowType string `json:"workflow_type"`
}

type OpenAPIApproveRequest struct {
	StageName    string `json:"stage_name"`
	WorkflowName string `json:"workflow_key"`
	TaskID       int64  `json:"task_id"`
	Approve      bool   `json:"approve"`
	Comment      string `json:"comment"`
}

type OpenAPICreateWorkflowViewReq struct {
	ProjectName  string                       `json:"project_key"`
	Name         string                       `json:"name"`
	WorkflowList []*OpenAPIWorkflowViewDetail `json:"workflow_list"`
}

type OpenAPIWorkflowViewDetail struct {
	WorkflowName        string `json:"workflow_key"`
	WorkflowDisplayName string `json:"workflow_name"`
	WorkflowType        string `json:"workflow_type"`
	Enabled             bool   `json:"enabled"`
}

func (req *OpenAPICreateWorkflowViewReq) Validate() (bool, error) {
	if req.ProjectName == "" {
		return false, fmt.Errorf("projectKey cannot be empty")
	}
	if req.Name == "" {
		return false, fmt.Errorf("view name cannot be empty")
	}

	for _, workflow := range req.WorkflowList {
		if workflow.WorkflowType != "product" && workflow.WorkflowType != "custom" {
			return false, fmt.Errorf("workflow type must be custom or product")
		}
	}
	return true, nil
}

type ApprovalJobInput struct {
}

func (p *ApprovalJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	return job, nil
}
