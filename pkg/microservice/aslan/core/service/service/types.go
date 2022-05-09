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

import (
	"encoding/json"
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
)

type LoadSource string

const (
	LoadFromRepo          LoadSource = "repo" //exclude gerrit
	LoadFromGerrit        LoadSource = "gerrit"
	LoadFromPublicRepo    LoadSource = "publicRepo"
	LoadFromChartTemplate LoadSource = "chartTemplate"
	LoadFromChartRepo     LoadSource = "chartRepo"
)

type HelmLoadSource struct {
	Source LoadSource `json:"source"`
}

type HelmServiceCreationArgs struct {
	HelmLoadSource
	Name           string                  `json:"name"`
	CreatedBy      string                  `json:"createdBy"`
	RequestID      string                  `json:"-"`
	AutoSync       bool                    `json:"auto_sync"`
	CreateFrom     interface{}             `json:"createFrom"`
	ValuesData     *service.ValuesDataArgs `json:"valuesData"`
	CreationDetail interface{}             `json:"-"`
}

type BulkHelmServiceCreationArgs struct {
	HelmLoadSource
	CreateFrom interface{}             `json:"createFrom"`
	CreatedBy  string                  `json:"createdBy"`
	RequestID  string                  `json:"-"`
	ValuesData *service.ValuesDataArgs `json:"valuesData"`
	AutoSync   bool                    `json:"auto_sync"`
}

type FailedService struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type BulkHelmServiceCreationResponse struct {
	SuccessServices []string         `json:"successServices"`
	FailedServices  []*FailedService `json:"failedServices"`
}

type CreateFromRepo struct {
	CodehostID int      `json:"codehostID"`
	Owner      string   `json:"owner"`
	Namespace  string   `json:"namespace"`
	Repo       string   `json:"repo"`
	Branch     string   `json:"branch"`
	Paths      []string `json:"paths"`
}

type CreateFromPublicRepo struct {
	RepoLink string   `json:"repoLink"`
	Paths    []string `json:"paths"`
}

type CreateFromChartTemplate struct {
	TemplateName string      `json:"templateName"`
	ValuesYAML   string      `json:"valuesYAML"`
	Variables    []*Variable `json:"variables"`
}

type CreateFromChartRepo struct {
	ChartRepoName string `json:"chartRepoName"`
	ChartName     string `json:"chartName"`
	ChartVersion  string `json:"chartVersion"`
}

func PublicRepoToPrivateRepoArgs(args *CreateFromPublicRepo) (*CreateFromRepo, error) {
	if args.RepoLink == "" {
		return nil, fmt.Errorf("empty link")
	}
	owner, repo, err := git.ParseOwnerAndRepo(args.RepoLink)
	if err != nil {
		return nil, err
	}

	return &CreateFromRepo{
		Owner: owner,
		Repo:  repo,
		Paths: args.Paths,
	}, nil
}

func (a *HelmServiceCreationArgs) UnmarshalJSON(data []byte) error {
	s := &HelmLoadSource{}
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}

	switch s.Source {
	case LoadFromRepo:
		a.CreateFrom = &CreateFromRepo{}
	case LoadFromPublicRepo:
		a.CreateFrom = &CreateFromPublicRepo{}
	case LoadFromChartTemplate:
		a.CreateFrom = &CreateFromChartTemplate{}
	case LoadFromChartRepo:
		a.CreateFrom = &CreateFromChartRepo{}
	}

	type tmp HelmServiceCreationArgs

	return json.Unmarshal(data, (*tmp)(a))
}

func (a *BulkHelmServiceCreationArgs) UnmarshalJSON(data []byte) error {
	s := &HelmLoadSource{}
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}

	if s.Source == LoadFromChartTemplate {
		a.CreateFrom = &CreateFromChartTemplate{}
	}

	type tmp BulkHelmServiceCreationArgs

	return json.Unmarshal(data, (*tmp)(a))
}
