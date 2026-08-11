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
	chartTemplates, err := ListChartTemplates(logger)
	if err != nil {
		return nil, err
	}

	return buildOpenAPIListChartTemplatesResponse(chartTemplates), nil
}

func buildOpenAPIListChartTemplatesResponse(chartTemplates *ChartTemplateListResp) *OpenAPIListChartTemplatesResponse {
	resp := &OpenAPIListChartTemplatesResponse{
		SystemVariables: make([]*OpenAPIChartVariable, 0, len(chartTemplates.SystemVariables)),
		ChartTemplates:  make([]*OpenAPIChartTemplate, 0, len(chartTemplates.ChartTemplates)),
	}

	for _, variable := range chartTemplates.SystemVariables {
		resp.SystemVariables = append(resp.SystemVariables, &OpenAPIChartVariable{
			Key:         variable.Key,
			Description: variable.Description,
		})
	}
	sort.Slice(resp.SystemVariables, func(i, j int) bool {
		return resp.SystemVariables[i].Key < resp.SystemVariables[j].Key
	})

	for _, chart := range chartTemplates.ChartTemplates {
		resp.ChartTemplates = append(resp.ChartTemplates, &OpenAPIChartTemplate{
			Name:       chart.Name,
			CodeHostID: chart.CodehostID,
			Owner:      chart.Owner,
			Namespace:  chart.Namespace,
			Repo:       chart.Repo,
			Path:       chart.Path,
			Branch:     chart.Branch,
		})
	}

	return resp
}
