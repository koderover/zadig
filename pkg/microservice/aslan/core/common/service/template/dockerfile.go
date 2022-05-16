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

package template

import (
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	dockerfileinstructions "github.com/moby/buildkit/frontend/dockerfile/instructions"
	dockerfileparser "github.com/moby/buildkit/frontend/dockerfile/parser"
)

func GetDockerfileTemplateDetail(id string, logger *zap.SugaredLogger) (*DockerfileDetail, error) {
	resp := new(DockerfileDetail)
	dockerfileTemplate, err := commonrepo.NewDockerfileTemplateColl().GetById(id)
	if err != nil {
		logger.Errorf("Failed to get dockerfile template from id: %s, the error is: %s", id, err)
		return nil, err
	}
	variables, err := getVariables(dockerfileTemplate.Content, logger)
	if err != nil {
		return nil, errors.New("failed to get variables from dockerfile")
	}
	resp.ID = dockerfileTemplate.ID.Hex()
	resp.Name = dockerfileTemplate.Name
	resp.Content = dockerfileTemplate.Content
	resp.Variables = variables
	return resp, nil
}

func getVariables(s string, logger *zap.SugaredLogger) ([]*commonmodels.ChartVariable, error) {
	ret := make([]*commonmodels.ChartVariable, 0)
	reader := strings.NewReader(s)
	result, err := dockerfileparser.Parse(reader)
	if err != nil {
		logger.Errorf("Failed to parse the dockerfile from source, the error is: %s", err)
		return []*commonmodels.ChartVariable{}, err
	}
	stages, metaArgs, err := dockerfileinstructions.Parse(result.AST)
	if err != nil {
		logger.Errorf("Failed to parse stages from generated dockerfile AST, the error is: %s", err)
		return []*commonmodels.ChartVariable{}, err
	}
	keyMap := make(map[string]int)
	for _, metaArg := range metaArgs {
		for _, arg := range metaArg.Args {
			if keyMap[arg.Key] == 0 {
				ret = append(ret, &commonmodels.ChartVariable{
					Key:   arg.Key,
					Value: arg.ValueString(),
				})
				keyMap[arg.Key] = 1
			}
		}
	}
	for _, stage := range stages {
		for _, command := range stage.Commands {
			if command.Name() == setting.DockerfileCmdArg {
				fullCommand := fmt.Sprintf("%s", command)
				commandContent := strings.Split(fullCommand, " ")[1]
				kv := strings.Split(commandContent, "=")
				key := kv[0]
				value := ""
				if len(kv) > 1 {
					value = kv[1]
				}
				// if key has not been added yet
				if keyMap[key] == 0 {
					ret = append(ret, &commonmodels.ChartVariable{
						Key:   key,
						Value: value,
					})
					keyMap[key] = 1
				}
			}
		}
	}
	return ret, nil
}
