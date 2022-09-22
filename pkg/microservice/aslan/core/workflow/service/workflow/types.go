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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
)

type WorkflowV3 struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	ProjectName string                   `json:"project_name"`
	Description string                   `json:"description"`
	Parameters  []*ParameterSetting      `json:"parameters"`
	SubTasks    []map[string]interface{} `json:"sub_tasks"`
}

type WorkflowV3Brief struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProjectName string `json:"project_name"`
}

type ParameterSettingType string

const (
	StringType   ParameterSettingType = "string"
	ChoiceType   ParameterSettingType = "choice"
	ExternalType ParameterSettingType = "external"
)

type ParameterSetting struct {
	// External type parameter will NOT use this key.
	Key  string               `json:"key"`
	Type ParameterSettingType `json:"type"`
	//DefaultValue is the
	DefaultValue string `json:"default_value"`
	// choiceOption Are all options enumerated
	ChoiceOption []string `json:"choice_option"`
	// ExternalSetting It is the configuration of the external system to obtain the variable
	ExternalSetting *ExternalSetting `json:"external_setting"`
}

type ExternalSetting struct {
	SystemID string                  `json:"system_id"`
	Endpoint string                  `json:"endpoint"`
	Method   string                  `json:"method"`
	Headers  []*KV                   `json:"headers"`
	Body     string                  `json:"body"`
	Params   []*ExternalParamMapping `json:"params"`
}

type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ExternalParamMapping struct {
	// zadig变量名称
	ParamKey string `json:"param_key"`
	// 返回中的key的位置
	ResponseKey string `json:"response_key"`
	Display     bool   `json:"display"`
}

type WorkflowV3TaskArgs struct {
	Type    string                   `json:"type"`
	Key     string                   `json:"key,omitempty"`
	Value   string                   `json:"value,omitempty"`
	Choice  []string                 `json:"choice,omitempty"`
	Options []map[string]interface{} `json:"options,omitempty"`
}

type OpenAPICreateCustomWorkflowTaskArgs struct {
	WorkflowName string                      `json:"workflow_name"`
	ProjectName  string                      `json:"project_name"`
	Inputs       []*CreateCustomTaskJobInput `json:"inputs"`
}

type CreateCustomTaskJobInput struct {
	JobName    string         `json:"job_name"`
	JobType    config.JobType `json:"job_type"`
	Parameters interface{}    `json:"parameters"`
}

type CustomJobInput interface {
	UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error)
}

type PluginJobInput struct {
	KVs []*KV `json:"kv"`
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

type FreestyleJobInput struct {
	KVs []*KV `json:"kv"`
}

func (p *FreestyleJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.FreestyleJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.PluginJobSpec")
	}
	kvMap := make(map[string]string)
	for _, kv := range p.KVs {
		kvMap[kv.Key] = kv.Value
	}

	for _, env := range newSpec.Properties.Envs {
		if val, ok := kvMap[env.Key]; ok {
			env.Value = val
		}
	}

	job.Spec = newSpec

	return job, nil
}

type ZadigBuildJobInput struct {
	Registry    string              `json:"registry"`
	ServiceList []*ServiceBuildArgs `json:"service_list"`
}

type ServiceBuildArgs struct {
	ServiceModule string       `json:"service_module"`
	ServiceName   string       `json:"service_name"`
	RepoInfo      []*RepoInput `json:"repo_info"`
	Inputs        []*KV        `json:"inputs"`
}

type RepoInput struct {
	CodeHostName  string `json:"codehost_name"`
	RepoNamespace string `json:"repo_namespace"`
	RepoName      string `json:"repo_name"`
	Branch        string `json:"branch"`
	PR            int    `json:"pr"`
}

func (p *ZadigBuildJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.ZadigBuildJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.PluginJobSpec")
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

	for _, svcBuild := range newSpec.ServiceAndBuilds {
		for _, inputSvc := range p.ServiceList {
			// if the service & service module match, we do the update logic
			if inputSvc.ServiceName == svcBuild.ServiceName && inputSvc.ServiceModule == svcBuild.ServiceModule {
				// update build repo info with input build info
				for _, inputRepo := range inputSvc.RepoInfo {
					repoInfo, err := mongodb.NewCodehostColl().GetCodeHostByAlias(inputRepo.CodeHostName)
					if err != nil {
						return nil, errors.New("failed to find code host with name:" + inputRepo.CodeHostName)
					}

					for _, buildRepo := range svcBuild.Repos {
						if buildRepo.CodehostID == repoInfo.ID {
							if buildRepo.RepoNamespace == inputRepo.RepoNamespace && buildRepo.RepoName == inputRepo.RepoName {
								buildRepo.Branch = inputRepo.Branch
								buildRepo.PR = inputRepo.PR
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
			}
		}
	}

	job.Spec = newSpec

	return job, nil
}

type ZadigDeployJobInput struct {
	EnvName     string               `json:"env_name"` // required
	ServiceList []*ServiceDeployArgs `json:"service_list"`
}

type ServiceDeployArgs struct {
	ServiceModule string `json:"service_module"`
	ServiceName   string `json:"service_name"`
	ImageName     string `json:"image_name"`
}

func (p *ZadigDeployJobInput) UpdateJobSpec(job *commonmodels.Job) (*commonmodels.Job, error) {
	newSpec := new(commonmodels.ZadigDeployJobSpec)
	if err := commonmodels.IToi(job.Spec, newSpec); err != nil {
		return nil, errors.New("unable to cast job.Spec into commonmodels.PluginJobSpec")
	}

	newSpec.Env = p.EnvName

	for _, svcDeploy := range newSpec.ServiceAndImages {
		for _, inputSvc := range p.ServiceList {
			if inputSvc.ServiceName == svcDeploy.ServiceName && inputSvc.ServiceModule == svcDeploy.ServiceModule {
				svcDeploy.Image = inputSvc.ImageName
			}
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
		return nil, errors.New("unable to cast job.Spec into commonmodels.PluginJobSpec")
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
		return nil, errors.New("unable to cast job.Spec into commonmodels.PluginJobSpec")
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
		return nil, errors.New("unable to cast job.Spec into commonmodels.PluginJobSpec")
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
