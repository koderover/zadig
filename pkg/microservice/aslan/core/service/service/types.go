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

package service

import "encoding/json"

type LoadSource string

const (
	LoadFromRepo          LoadSource = "repo"
	LoadFromChartTemplate LoadSource = "chartTemplate"
)

type HelmLoadSource struct {
	Source LoadSource `json:"source"`
}

type HelmServiceCreationArgs struct {
	HelmLoadSource

	Name       string      `json:"name"`
	CreatedBy  string      `json:"createdBy"`
	CreateFrom interface{} `json:"createFrom"`
}

type CreateFromRepo struct {
	CodehostID int      `json:"codehostID"`
	Owner      string   `json:"owner"`
	Repo       string   `json:"repo"`
	Branch     string   `json:"branch"`
	Paths      []string `json:"paths"`
}

type CreateFromChartTemplate struct {
	CodehostID   int      `json:"codehostID"`
	Owner        string   `json:"owner"`
	Repo         string   `json:"repo"`
	Branch       string   `json:"branch"`
	TemplateName string   `json:"templateName"`
	ValuesPaths  []string `json:"valuesPaths"`
	ValuesYAML   string   `json:"valuesYAML"`
}

func (a *HelmServiceCreationArgs) UnmarshalJSON(data []byte) error {
	s := &HelmLoadSource{}
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}

	switch s.Source {
	case LoadFromRepo:
		a.CreateFrom = &CreateFromRepo{}
	case LoadFromChartTemplate:
		a.CreateFrom = &CreateFromChartTemplate{}
	}

	type tmp HelmServiceCreationArgs

	return json.Unmarshal(data, (*tmp)(a))
}
