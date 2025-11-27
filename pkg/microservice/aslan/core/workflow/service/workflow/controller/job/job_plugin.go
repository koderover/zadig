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
	"strings"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

type PluginJobController struct {
	*BasicInfo

	jobSpec *commonmodels.PluginJobSpec
}

func CreatePluginJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.PluginJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:          job.Name,
		jobType:       job.JobType,
		errorPolicy:   job.ErrorPolicy,
		executePolicy: job.ExecutePolicy,
		workflow:      workflow,
	}

	return PluginJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j PluginJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j PluginJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j PluginJobController) Validate(isExecution bool) error {
	return checkOutputNames(j.jobSpec.Plugin.Outputs)
}

func (j PluginJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.PluginJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	newInputs := renderParams(currJobSpec.Plugin.Inputs, j.jobSpec.Plugin.Inputs)
	j.jobSpec.Plugin = currJobSpec.Plugin
	j.jobSpec.Plugin.Inputs = newInputs
	return nil
}

func (j PluginJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j PluginJobController) ClearOptions() {
	return
}

func (j PluginJobController) ClearSelection() {
	return
}

func (j PluginJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	registries, err := commonservice.ListRegistryNamespaces("", true, log.SugaredLogger())
	if err != nil {
		return resp, err
	}

	taskRunProperties := &commonmodels.JobProperties{
		Timeout:             j.jobSpec.AdvancedSetting.Timeout,
		ResourceRequest:     j.jobSpec.AdvancedSetting.ResourceRequest,
		ResReqSpec:          j.jobSpec.AdvancedSetting.ResReqSpec,
		ClusterID:           j.jobSpec.AdvancedSetting.ClusterID,
		ClusterSource:       j.jobSpec.AdvancedSetting.ClusterSource,
		StrategyID:          j.jobSpec.AdvancedSetting.StrategyID,
		ShareStorageDetails: getShareStorageDetail(j.workflow.ShareStorages, j.jobSpec.AdvancedSetting.ShareStorageInfo, j.workflow.Name, taskID),
		UseHostDockerDaemon: j.jobSpec.AdvancedSetting.UseHostDockerDaemon,
		Registries:          registries,
		CustomAnnotations:   j.jobSpec.AdvancedSetting.CustomAnnotations,
		CustomLabels:        j.jobSpec.AdvancedSetting.CustomLabels,
	}

	jobTaskSpec := &commonmodels.JobTaskPluginSpec{
		Properties: *taskRunProperties,
		Plugin:     j.jobSpec.Plugin,
	}
	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType:       string(config.JobPlugin),
		Spec:          jobTaskSpec,
		Outputs:       j.jobSpec.Plugin.Outputs,
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,
	}

	renderedParams := []*commonmodels.Param{}
	for _, param := range j.jobSpec.Plugin.Inputs {
		paramsKey := strings.Join([]string{"inputs", param.Name}, ".")
		renderedParams = append(renderedParams, &commonmodels.Param{Name: paramsKey, Value: param.Value, ParamsType: "string", IsCredential: false})
	}
	jobTaskSpec.Plugin = renderPlugin(jobTaskSpec.Plugin, renderedParams)

	jobTask.Outputs = j.jobSpec.Plugin.Outputs
	resp = append(resp, jobTask)
	return resp, nil
}

func (j PluginJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j PluginJobController) SetRepoCommitInfo() error {
	return nil
}

func (j PluginJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)
	if getRuntimeVariables {
		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "status"}, "."),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})
	}
	return resp, nil
}

func (j PluginJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j PluginJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j PluginJobController) IsServiceTypeJob() bool {
	return false
}

func renderPlugin(plugin *commonmodels.PluginTemplate, inputs []*commonmodels.Param) *commonmodels.PluginTemplate {
	for _, env := range plugin.Envs {
		env.Value = renderString(env.Value, setting.RenderPluginValueTemplate, inputs)
	}
	for i, arg := range plugin.Args {
		plugin.Args[i] = renderString(arg, setting.RenderPluginValueTemplate, inputs)
	}
	for i, cmd := range plugin.Cmds {
		plugin.Cmds[i] = renderString(cmd, setting.RenderPluginValueTemplate, inputs)
	}
	return plugin
}
