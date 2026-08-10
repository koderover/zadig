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
	"sort"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

type OpenAPIListChartTemplatesResponse struct {
	SystemVariables []*OpenAPIChartVariable `json:"system_variables"`
	ChartTemplates  []*OpenAPIChartTemplate `json:"chart_templates"`
}

type OpenAPIChartVariable struct {
	Key         string `json:"key"`
	Description string `json:"description,omitempty"`
}

type OpenAPIChartTemplate struct {
	Name       string `json:"name"`
	CodeHostID int    `json:"codehost_id"`
	Owner      string `json:"owner"`
	Namespace  string `json:"namespace"`
	Repo       string `json:"repo"`
	Path       string `json:"path"`
	Branch     string `json:"branch"`
}

func OpenAPIListChartTemplates(logger *zap.SugaredLogger) (*OpenAPIListChartTemplatesResponse, error) {
	charts, err := listChartTemplates(logger)
	if err != nil {
		return nil, err
	}

	return buildOpenAPIListChartTemplatesResponse(charts), nil
}

func buildOpenAPIListChartTemplatesResponse(charts []*commonmodels.Chart) *OpenAPIListChartTemplatesResponse {
	resp := &OpenAPIListChartTemplatesResponse{
		SystemVariables: make([]*OpenAPIChartVariable, 0, len(ChartTemplateDefaultSystemVariable)),
		ChartTemplates:  make([]*OpenAPIChartTemplate, 0, len(charts)),
	}

	variableKeys := make([]string, 0, len(ChartTemplateDefaultSystemVariable))
	for key := range ChartTemplateDefaultSystemVariable {
		variableKeys = append(variableKeys, key)
	}
	sort.Strings(variableKeys)
	for _, key := range variableKeys {
		resp.SystemVariables = append(resp.SystemVariables, &OpenAPIChartVariable{
			Key:         key,
			Description: ChartTemplateDefaultSystemVariable[key],
		})
	}

	for _, chart := range charts {
		resp.ChartTemplates = append(resp.ChartTemplates, &OpenAPIChartTemplate{
			Name:       chart.Name,
			CodeHostID: chart.CodeHostID,
			Owner:      chart.Owner,
			Namespace:  chart.GetNamespace(),
			Repo:       chart.Repo,
			Path:       chart.Path,
			Branch:     chart.Branch,
		})
	}

	return resp
}
