/*
Copyright 2025 The KodeRover Authors.
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

package job

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type UpdateEnvIstioConfigJobController struct {
	*BasicInfo

	jobSpec *commonmodels.UpdateEnvIstioConfigJobSpec
}

func CreateUpdateEnvIstioConfigJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.UpdateEnvIstioConfigJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return UpdateEnvIstioConfigJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j UpdateEnvIstioConfigJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j UpdateEnvIstioConfigJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j UpdateEnvIstioConfigJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	if j.jobSpec.GrayscaleStrategy == commonmodels.GrayscaleStrategyWeight {
		weight := int32(0)
		for _, config := range j.jobSpec.WeightConfigs {
			weight += config.Weight
		}
		if weight != 100 {
			return fmt.Errorf("weight sum should be 100, but got %d", weight)
		}
	}
	return nil
}

func (j UpdateEnvIstioConfigJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.UpdateEnvIstioConfigJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.Production = currJobSpec.Production
	j.jobSpec.BaseEnv = currJobSpec.BaseEnv
	j.jobSpec.Source = currJobSpec.Source
	if currJobSpec.Source == "fixed" {
		j.jobSpec.WeightConfigs = currJobSpec.WeightConfigs
		j.jobSpec.HeaderMatchConfigs = currJobSpec.HeaderMatchConfigs
	} else {
		// otherwise do a merge
		if j.jobSpec.GrayscaleStrategy == commonmodels.GrayscaleStrategyWeight {
			newWeightStrat := make([]commonmodels.IstioWeightConfig, 0)
			foundInUserInput := false
			var baseWeight int32
			// for weight it now only supports 2 items, so we do a tricky merge
			for _, userInput := range j.jobSpec.WeightConfigs {
				if userInput.Env == j.jobSpec.BaseEnv {
					newWeightStrat = append(newWeightStrat, commonmodels.IstioWeightConfig{
						Env:    j.jobSpec.BaseEnv,
						Weight: userInput.Weight,
					})
					foundInUserInput = true
					baseWeight = userInput.Weight
					break
				}
			}

			for _, configuredEnv := range currJobSpec.WeightConfigs {
				if configuredEnv.Env == j.jobSpec.BaseEnv {
					if !foundInUserInput {
						newWeightStrat = append(newWeightStrat, commonmodels.IstioWeightConfig{
							Env:    j.jobSpec.BaseEnv,
							Weight: configuredEnv.Weight,
						})
					}
				} else {
					newWeight := configuredEnv.Weight
					if foundInUserInput {
						newWeight = 100 - baseWeight
					}
					newWeightStrat = append(newWeightStrat, commonmodels.IstioWeightConfig{
						Env:    configuredEnv.Env,
						Weight: newWeight,
					})
				}
			}
		} else {
			userInputMap := make(map[string][]commonmodels.IstioHeaderMatch)
			for _, input := range j.jobSpec.HeaderMatchConfigs {
				userInputMap[input.Env] = input.HeaderMatchs
			}

			for _, configuredMatch := range currJobSpec.HeaderMatchConfigs {
				input, ok := userInputMap[configuredMatch.Env]
				if !ok {
					continue
				}

				userInputKVMap := make(map[string]commonmodels.IstioHeaderMatch)
				for _, kv := range input {
					userInputKVMap[kv.Key] = kv
				}

				newMatchingKV := make([]commonmodels.IstioHeaderMatch, 0)

				for _, configuredKV := range configuredMatch.HeaderMatchs {
					if inputKV, ok := userInputKVMap[configuredKV.Key]; ok {
						newMatchingKV = append(newMatchingKV, commonmodels.IstioHeaderMatch{
							Key:   configuredKV.Key,
							Value: inputKV.Value,
							Match: inputKV.Match,
						})
					} else {
						newMatchingKV = append(newMatchingKV, configuredKV)
					}
				}

				configuredMatch.HeaderMatchs = newMatchingKV
			}
		}
	}

	return nil
}

func (j UpdateEnvIstioConfigJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j UpdateEnvIstioConfigJobController) ClearOptions() {
	return
}

func (j UpdateEnvIstioConfigJobController) ClearSelection() {
	return
}

func (j UpdateEnvIstioConfigJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType:     string(config.JobUpdateEnvIstioConfig),
		Spec:        j.jobSpec,
		ErrorPolicy: j.errorPolicy,
	}
	resp = append(resp, jobTask)

	return resp, nil
}

func (j UpdateEnvIstioConfigJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j UpdateEnvIstioConfigJobController) SetRepoCommitInfo() error {
	return nil
}

func (j UpdateEnvIstioConfigJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j UpdateEnvIstioConfigJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j UpdateEnvIstioConfigJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}
