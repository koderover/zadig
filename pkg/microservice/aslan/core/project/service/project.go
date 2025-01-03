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
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
)

type QueryVerbosity string

const (
	VerbosityDetailed QueryVerbosity = "detailed" // all information
	VerbosityBrief    QueryVerbosity = "brief"    // short information or a summary
	VerbosityMinimal  QueryVerbosity = "minimal"  // very little information, usually only a resource identifier
)

type ProjectListOptions struct {
	IgnoreNoEnvs     bool
	IgnoreNoVersions bool
	Verbosity        QueryVerbosity
	Names            []string
	PageSize         int64
	PageNum          int64
	Filter           string
	GroupName        string
	Ungrouped        bool
}
type ProjectDetailedResponse struct {
	ProjectDetailedRepresentation []*ProjectDetailedRepresentation `json:"projects"`
	Total                         int                              `json:"total"`
}

type ProjectDetailedRepresentation struct {
	*ProjectBriefRepresentation
	Alias      string `json:"alias"`
	Desc       string `json:"desc"`
	UpdatedAt  int64  `json:"updatedAt"`
	UpdatedBy  string `json:"updatedBy"`
	Onboard    bool   `json:"onboard"`
	Public     bool   `json:"public"`
	DeployType string `json:"deployType"`
}

type ProjectBriefResponse struct {
	ProjectBriefRepresentation []*ProjectBriefRepresentation `json:"projects"`
	Total                      int                           `json:"total"`
}

type ProjectBriefRepresentation struct {
	*ProjectMinimalRepresentation
	Envs []string `json:"envs"`
}

type ProjectMinimalRepresentation struct {
	Name string `json:"name"`
}

func ListProjects(opts *ProjectListOptions, logger *zap.SugaredLogger) (interface{}, error) {
	authorizedProjectList := sets.NewString(opts.Names...)
	projectKeys := make([]string, 0)

	var err error
	if opts.Ungrouped {
		projectKeys, err = GetUnGroupedProjectKeys()
		if err != nil {
			msg := fmt.Errorf("failed to list ungrouped projects, err: %s", err)
			logger.Error(msg)
			return nil, msg
		}

		if len(projectKeys) == 0 {
			return &ProjectDetailedResponse{
				ProjectDetailedRepresentation: nil,
				Total:                         0,
			}, nil
		}
	} else if opts.GroupName != "" {
		group, err := mongodb.NewProjectGroupColl().Find(mongodb.ProjectGroupOpts{Name: opts.GroupName})
		if err != nil && (err != mongo.ErrNoDocuments && err != mongo.ErrNilDocument) {
			logger.Errorf("Failed to list projects, err: %s", err)
			return nil, err
		}

		// if the project group does not hava any projects, return empty
		if group != nil && len(group.Projects) == 0 {
			return &ProjectDetailedResponse{
				ProjectDetailedRepresentation: nil,
				Total:                         0,
			}, nil
		}

		if group != nil && group.Projects != nil {
			for _, project := range group.Projects {
				projectKeys = append(projectKeys, project.ProjectKey)
			}
		}
	}

	if len(projectKeys) > 0 && len(authorizedProjectList) > 0 {
		opts.Names = authorizedProjectList.Intersection(sets.NewString(projectKeys...)).List()
		if len(opts.Names) == 0 {
			return &ProjectDetailedResponse{
				ProjectDetailedRepresentation: nil,
				Total:                         0,
			}, nil
		}
	} else if len(projectKeys) == 0 && len(authorizedProjectList) > 0 {
		opts.Names = authorizedProjectList.List()
	} else {
		opts.Names = projectKeys
	}

	switch opts.Verbosity {
	case VerbosityDetailed:
		return listDetailedProjectInfos(opts, logger)
	case VerbosityBrief:
		return listBriefProjectInfos(opts, logger)
	case VerbosityMinimal:
		return listMinimalProjectInfos(opts, logger)
	default:
		return listMinimalProjectInfos(opts, logger)
	}
}

func listDetailedProjectInfos(opts *ProjectListOptions, logger *zap.SugaredLogger) (*ProjectDetailedResponse, error) {
	var representation []*ProjectDetailedRepresentation

	nameOrder, nameSet, nameMap, total, err := getProjects(opts)
	if err != nil {
		logger.Errorf("Failed to list projects, err: %s", err)
		return nil, err
	}

	newOpts := &ProjectListOptions{
		Names: nameSet.List(),
	}
	nameWithEnvSet, nameWithEnvMap, err := getProjectsWithEnvs(newOpts)
	if err != nil {
		logger.Errorf("Failed to list projects, err: %s", err)
		return nil, err
	}

	desiredSet := nameSet
	if opts.IgnoreNoEnvs {
		desiredSet = nameSet.Intersection(nameWithEnvSet)
	}

	for _, name := range nameOrder {
		if !desiredSet.Has(name) {
			continue
		}

		info := nameMap[name]
		deployType := info.ProductFeature.GetDeployType()
		representation = append(representation, &ProjectDetailedRepresentation{
			ProjectBriefRepresentation: &ProjectBriefRepresentation{
				ProjectMinimalRepresentation: &ProjectMinimalRepresentation{Name: name},
				Envs:                         nameWithEnvMap[name],
			},
			Alias:      info.Alias,
			Desc:       info.Desc,
			UpdatedAt:  info.UpdatedAt,
			UpdatedBy:  info.UpdatedBy,
			Onboard:    info.OnboardStatus != 0,
			Public:     info.Public,
			DeployType: deployType,
		})
	}

	res := &ProjectDetailedResponse{
		ProjectDetailedRepresentation: representation,
		Total:                         total,
	}

	return res, nil
}

func listBriefProjectInfos(opts *ProjectListOptions, logger *zap.SugaredLogger) (*ProjectBriefResponse, error) {
	var representation []*ProjectBriefRepresentation

	nameOrder, nameSet, _, total, err := getProjects(opts)
	if err != nil {
		logger.Errorf("Failed to list projects, err: %s", err)
		return nil, err
	}

	newOpts := &ProjectListOptions{
		Names: nameSet.List(),
	}
	nameWithEnvSet, nameWithEnvMap, err := getProjectsWithEnvs(newOpts)
	if err != nil {
		logger.Errorf("Failed to list projects, err: %s", err)
		return nil, err
	}

	desiredSet := nameSet
	if opts.IgnoreNoEnvs {
		desiredSet = nameSet.Intersection(nameWithEnvSet)
	}

	for _, name := range nameOrder {
		if !desiredSet.Has(name) {
			continue
		}

		representation = append(representation, &ProjectBriefRepresentation{
			ProjectMinimalRepresentation: &ProjectMinimalRepresentation{Name: name},
			Envs:                         nameWithEnvMap[name],
		})
	}

	res := &ProjectBriefResponse{
		ProjectBriefRepresentation: representation,
		Total:                      total,
	}

	return res, nil
}

func listMinimalProjectInfoForDelivery(_ *ProjectListOptions, nameSet sets.String, logger *zap.SugaredLogger) ([]*ProjectMinimalRepresentation, error) {
	var res []*ProjectMinimalRepresentation
	namesWithDelivery, err := mongodb.NewDeliveryVersionColl().FindProducts()
	if err != nil {
		logger.Errorf("Failed to list projects by delivery, err: %s", err)
		return nil, err
	}

	for _, name := range namesWithDelivery {
		// namesWithDelivery may contain projects which are already deleted.
		if !nameSet.Has(name) {
			continue
		}
		res = append(res, &ProjectMinimalRepresentation{Name: name})
	}

	return res, nil
}

func listMinimalProjectInfos(opts *ProjectListOptions, logger *zap.SugaredLogger) ([]*ProjectMinimalRepresentation, error) {
	var res []*ProjectMinimalRepresentation
	names, err := templaterepo.NewProductColl().ListNames(opts.Names)
	if err != nil {
		logger.Errorf("Failed to list project names, err: %s", err)
		return nil, err
	}

	nameSet := sets.NewString(names...)
	if opts.IgnoreNoVersions {
		return listMinimalProjectInfoForDelivery(opts, nameSet, logger)
	}

	if !opts.IgnoreNoEnvs {
		for _, name := range names {
			res = append(res, &ProjectMinimalRepresentation{Name: name})
		}

		return res, nil
	}

	nameWithEnvSet, _, err := getProjectsWithEnvs(opts)
	if err != nil {
		logger.Errorf("Failed to list projects, err: %s", err)
		return nil, err
	}

	for name := range nameWithEnvSet {
		// nameWithEnvs may contain projects which are already deleted.
		if !nameSet.Has(name) {
			continue
		}
		res = append(res, &ProjectMinimalRepresentation{Name: name})
	}

	return res, nil
}

func getProjectsWithEnvs(opts *ProjectListOptions) (sets.String, map[string][]string, error) {
	nameWithEnvs, err := mongodb.NewProductColl().ListProjectsInNames(opts.Names)
	if err != nil {
		return nil, nil, err
	}

	nameSet := sets.NewString()
	nameMap := make(map[string][]string)
	for _, nameWithEnv := range nameWithEnvs {
		nameSet.Insert(nameWithEnv.ProjectName)
		nameMap[nameWithEnv.ProjectName] = nameWithEnv.Envs
	}

	saeNameWithEnvs, err := mongodb.NewSAEEnvColl().ListProjectsInNames(opts.Names)
	if err != nil {
		return nil, nil, err
	}

	for _, nameWithEnv := range saeNameWithEnvs {
		nameSet.Insert(nameWithEnv.ProjectName)
		nameMap[nameWithEnv.ProjectName] = append(nameMap[nameWithEnv.ProjectName], nameWithEnv.Envs...)
	}

	return nameSet, nameMap, nil
}

func getProjects(opts *ProjectListOptions) ([]string, sets.String, map[string]*templaterepo.ProjectInfo, int, error) {
	listOpts := templaterepo.ProductListByFilterOpt{
		Names:  opts.Names,
		Filter: opts.Filter,
		Limit:  opts.PageSize,
		Skip:   (opts.PageNum - 1) * opts.PageSize,
	}
	res, total, err := templaterepo.NewProductColl().PageListProjectByFilter(listOpts)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	nameOrder := []string{}
	nameSet := sets.NewString()
	nameMap := make(map[string]*templaterepo.ProjectInfo)
	for _, r := range res {
		nameSet.Insert(r.Name)
		nameMap[r.Name] = r
		nameOrder = append(nameOrder, r.Name)
	}

	return nameOrder, nameSet, nameMap, total, nil
}
