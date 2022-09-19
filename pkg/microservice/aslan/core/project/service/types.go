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

package service

import (
	"errors"
	"regexp"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
)

type OpenAPICreateProductReq struct {
	ProjectName string             `json:"project_name"`
	ProjectKey  string             `json:"project_key"`
	IsPublic    bool               `json:"is_public"`
	Description string             `json:"description"`
	ProjectType config.ProjectType `json:"project_type"`
}

func (req OpenAPICreateProductReq) Validate() error {
	if req.ProjectName == "" {
		return errors.New("project_name cannot be empty")
	}

	match, err := regexp.MatchString(setting.ProjectKeyRegEx, req.ProjectKey)
	if err != nil || !match {
		return errors.New(`project key should match regex: ^[a-z-\\d]+$`)
	}

	switch req.ProjectType {
	case config.ProjectTypeLoaded, config.ProjectTypeYaml, config.ProjectTypeHelm, config.ProjectTypeVM:
		break
	default:
		return errors.New("unsupported project type")
	}

	return nil
}
