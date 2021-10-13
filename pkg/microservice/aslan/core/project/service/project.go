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
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
)

type QueryVerbosity string

const (
	VerbosityDetailed QueryVerbosity = "detailed" // all information
	VerbosityBrief    QueryVerbosity = "brief"    // short information or a summary
	VerbosityMinimal  QueryVerbosity = "minimal"  // very little information, usually only a resource identifier
)

type ProjectListOptions struct {
	IgnoreNoEnvs bool
	Verbosity    QueryVerbosity
	Projects     []string
}

type ProjectDetailedRepresentation struct {
	*ProjectBriefRepresentation
	Alias string
	Desc  string
}

type ProjectBriefRepresentation struct {
	*ProjectMinimalRepresentation
	Envs []string `json:"envs"`
}

type ProjectMinimalRepresentation struct {
	Name string `json:"name"`
}

func ListProjects(opts *ProjectListOptions, logger *zap.SugaredLogger) (interface{}, error) {
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

func listDetailedProjectInfos(opts *ProjectListOptions, logger *zap.SugaredLogger) (res []*ProjectDetailedRepresentation, err error) {
	nameWithEnvs, err := mongodb.NewProductColl().ListProjects()
	if err != nil {
		logger.Errorf("Failed to list projects, err: %s", err)
		return nil, err
	}

	nameSet := sets.NewString()
	for _, name := range opts.Projects {
		nameSet.Insert(name)
	}

	nameWithEnvsSet := sets.NewString()
	for _, nameWithEnv := range nameWithEnvs {
		// nameWithEnvs may contain projects which are already deleted.
		if !nameSet.Has(nameWithEnv.ProjectName) {
			continue
		}
		res = append(res, &ProjectDetailedRepresentation{
			ProjectBriefRepresentation: &ProjectBriefRepresentation{
				ProjectMinimalRepresentation: &ProjectMinimalRepresentation{Name: nameWithEnv.ProjectName},
				Envs:                         nameWithEnv.Envs,
			},
		})
		nameWithEnvsSet.Insert(nameWithEnv.ProjectName)
	}

	projects, err := template.NewProductColl().ListProjectsByNames(opts.Projects)
	if err != nil {
		return nil, err
	}
	if !opts.IgnoreNoEnvs {
		for _, project := range projects {
			if !nameWithEnvsSet.Has(project.ProjectName) {
				res = append(res, &ProjectDetailedRepresentation{
					ProjectBriefRepresentation: &ProjectBriefRepresentation{
						ProjectMinimalRepresentation: &ProjectMinimalRepresentation{Name: project.ProjectName},
					},
					Alias: project.ProductName,
					Desc:  project.Description,
				})
			}
		}
	}

	return res, nil
}

func listBriefProjectInfos(opts *ProjectListOptions, logger *zap.SugaredLogger) (res []*ProjectBriefRepresentation, err error) {
	nameWithEnvs, err := mongodb.NewProductColl().ListProjects()
	if err != nil {
		logger.Errorf("Failed to list projects, err: %s", err)
		return nil, err
	}

	nameSet := sets.NewString()
	for _, name := range opts.Projects {
		nameSet.Insert(name)
	}

	nameWithEnvsSet := sets.NewString()
	for _, nameWithEnv := range nameWithEnvs {
		// nameWithEnvs may contain projects which are already deleted.
		if !nameSet.Has(nameWithEnv.ProjectName) {
			continue
		}
		res = append(res, &ProjectBriefRepresentation{
			ProjectMinimalRepresentation: &ProjectMinimalRepresentation{Name: nameWithEnv.ProjectName},
			Envs:                         nameWithEnv.Envs,
		})
		nameWithEnvsSet.Insert(nameWithEnv.ProjectName)
	}

	if !opts.IgnoreNoEnvs {
		for _, name := range opts.Projects {
			if !nameWithEnvsSet.Has(name) {
				res = append(res, &ProjectBriefRepresentation{
					ProjectMinimalRepresentation: &ProjectMinimalRepresentation{Name: name},
				})
			}
		}
	}

	return res, nil
}

func listMinimalProjectInfos(opts *ProjectListOptions, logger *zap.SugaredLogger) (res []*ProjectMinimalRepresentation, err error) {
	if !opts.IgnoreNoEnvs {
		for _, name := range opts.Projects {
			res = append(res, &ProjectMinimalRepresentation{Name: name})
		}

		return res, nil
	}

	nameSet := sets.NewString()
	for _, name := range opts.Projects {
		nameSet.Insert(name)
	}

	nameWithEnvs, err := mongodb.NewProductColl().ListProjects()
	if err != nil {
		logger.Errorf("Failed to list projects, err: %s", err)
		return nil, err
	}

	for _, nameWithEnv := range nameWithEnvs {
		// nameWithEnvs may contain projects which are already deleted.
		if !nameSet.Has(nameWithEnv.ProjectName) {
			continue
		}
		res = append(res, &ProjectMinimalRepresentation{Name: nameWithEnv.ProjectName})
	}

	return res, nil
}
