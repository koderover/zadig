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

package job

import (
	"strings"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/tool/log"
)

type PluginJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.PluginJobSpec
}

func (j *PluginJob) Instantiate() error {
	j.spec = &commonmodels.PluginJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *PluginJob) SetPreset() error {
	j.spec = &commonmodels.PluginJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *PluginJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	logger := log.SugaredLogger()
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.PluginJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	j.job.Spec = j.spec
	jobTask := &commonmodels.JobTask{
		Name:       j.job.Name,
		JobType:    string(config.JobPlugin),
		Properties: *j.spec.Properties,
		Plugin:     j.spec.Plugin,
	}
	registries, err := commonservice.ListRegistryNamespaces("", true, logger)
	if err != nil {
		return resp, err
	}
	jobTask.Properties.Registries = registries
	for _, output := range j.spec.Plugin.Outputs {
		jobTask.Outputs = append(jobTask.Outputs, &commonmodels.Output{Name: output.Name, Description: output.Description})
	}
	jobTask.Plugin = renderPlugin(jobTask.Plugin, j.spec.Plugin.Inputs)
	return []*commonmodels.JobTask{jobTask}, nil
}

func renderPlugin(plugin *commonmodels.PluginTemplate, inputs []*commonmodels.Params) *commonmodels.PluginTemplate {
	for _, env := range plugin.Envs {
		env.Value = renderString(env.Value, inputs)
	}
	for i, arg := range plugin.Args {
		plugin.Args[i] = renderString(arg, inputs)
	}
	for i, cmd := range plugin.Cmds {
		plugin.Cmds[i] = renderString(cmd, inputs)
	}
	return plugin
}

func renderString(value string, inputs []*commonmodels.Params) string {
	for _, input := range inputs {
		value = strings.ReplaceAll(value, "$("+input.Name+")", input.Value)
	}
	return value
}
