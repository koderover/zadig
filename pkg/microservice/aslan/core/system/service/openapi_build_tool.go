/*
Copyright 2026 The KodeRover Authors.

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
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/types"
)

type OpenAPIBuildToolList struct {
	BuildTools []*types.OpenAPIToolItem `json:"build_tools"`
}

func OpenAPIListBuildTools(logger *zap.SugaredLogger) (*OpenAPIBuildToolList, error) {
	installs, err := ListAvaiableInstalls(logger)
	if err != nil {
		return nil, err
	}
	resp := &OpenAPIBuildToolList{BuildTools: make([]*types.OpenAPIToolItem, 0, len(installs))}
	for _, install := range installs {
		resp.BuildTools = append(resp.BuildTools, &types.OpenAPIToolItem{Name: install.Name, Version: install.Version})
	}
	return resp, nil
}
