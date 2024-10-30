/*
Copyright 2024 The KodeRover Authors.

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
	"fmt"
	"strings"
	"text/template"

	"github.com/koderover/zadig/v2/pkg/util"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

func RenderWorkflowVariables(projectKey, workflowName, variableType, jobName, variableKey string, input []*commonmodels.KV, log *zap.SugaredLogger) ([]string, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		log.Errorf("failed to find workflow with name: %s in project: %s, error: %s", workflowName, projectKey, err)
		return nil, fmt.Errorf("failed to find workflow with name: %s in project: %s, error: %s", workflowName, projectKey, err)
	}

	userInput := make(map[string]string)
	// generate a user input map
	for _, kv := range input {
		// special logic: since when we call the function we use value instead of parameter, add quotation mark to the value, and escape the " character
		val := strings.ReplaceAll(kv.Value, `"`, `\"`)
		userInput[kv.Key] = fmt.Sprintf("\"%s\"", val)
	}

	switch variableType {
	case "job":
		for _, stage := range workflow.Stages {
			for _, job := range stage.Jobs {
				if job.Name == jobName {
					switch job.JobType {
					case config.JobFreestyle:
						spec := &commonmodels.FreestyleJobSpec{}
						if err := commonmodels.IToi(job.Spec, spec); err != nil {
							return nil, fmt.Errorf("failed to decode freestyle job spec, error: %s", err)
						}
						for _, kv := range spec.Properties.Envs {
							if kv.Key == variableKey {
								resp, err := renderScriptedVariableOptions(kv.Script, kv.CallFunction, userInput)
								if err != nil {
									log.Errorf("Failed to render kv for key: %s, error: %s", variableKey, err)
									return nil, err
								}
								return resp, nil
							}
						}
					default:
						return nil, fmt.Errorf("unsupported job type: %s", job.JobType)
					}
				}
			}
		}
	case "workflow":
		for _, kv := range workflow.KeyVals {
			if kv.Key == variableKey {
				resp, err := renderScriptedVariableOptions(kv.Script, kv.CallFunction, userInput)
				if err != nil {
					log.Errorf("Failed to render kv for key: %s, error: %s", variableKey, err)
					return nil, err
				}
				return resp, nil
			}
		}
	default:
		return nil, fmt.Errorf("unsupported variable type")
	}
	return nil, fmt.Errorf("key: %s not found in job: %s for workflow: %s", variableKey, jobName, workflowName)
}

func renderScriptedVariableOptions(script, callFunction string, userInput map[string]string) ([]string, error) {
	t, err := template.New("scriptRender").Parse(callFunction)
	if err != nil {
		return nil, err
	}

	var realCallFunc bytes.Buffer
	err = t.Execute(&realCallFunc, userInput)
	if err != nil {
		return nil, err
	}

	resp, err := util.RunScriptWithCallFunc(script, realCallFunc.String())
	if err != nil {
		return nil, err
	}

	return resp, nil
}
