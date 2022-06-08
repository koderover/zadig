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
	"github.com/hashicorp/go-multierror"
	"github.com/koderover/zadig/pkg/config"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
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

func listDetailedProjectInfos(opts *ProjectListOptions, logger *zap.SugaredLogger) ([]*ProjectDetailedRepresentation, error) {
	var res []*ProjectDetailedRepresentation

	nameSet, nameMap, err := getProjects(opts)
	if err != nil {
		logger.Errorf("Failed to list projects, err: %s", err)
		return nil, err
	}

	nameWithEnvSet, nameWithEnvMap, err := getProjectsWithEnvs(opts)
	if err != nil {
		logger.Errorf("Failed to list projects, err: %s", err)
		return nil, err
	}

	desiredSet := nameSet
	if opts.IgnoreNoEnvs {
		desiredSet = nameSet.Intersection(nameWithEnvSet)
	}

	for name := range desiredSet {
		info := nameMap[name]
		var deployType string
		if info.CreateEnvType == "external" {
			deployType = "external"
		} else if info.BasicFacility == "cloud_host" {
			deployType = "cloud_host"
		} else {
			deployType = info.DeployType
		}
		res = append(res, &ProjectDetailedRepresentation{
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

	return res, nil
}

func listBriefProjectInfos(opts *ProjectListOptions, logger *zap.SugaredLogger) ([]*ProjectBriefRepresentation, error) {
	var res []*ProjectBriefRepresentation

	nameSet, _, err := getProjects(opts)
	if err != nil {
		logger.Errorf("Failed to list projects, err: %s", err)
		return nil, err
	}

	nameWithEnvSet, nameWithEnvMap, err := getProjectsWithEnvs(opts)
	if err != nil {
		logger.Errorf("Failed to list projects, err: %s", err)
		return nil, err
	}

	desiredSet := nameSet
	if opts.IgnoreNoEnvs {
		desiredSet = nameSet.Intersection(nameWithEnvSet)
	}

	for name := range desiredSet {
		res = append(res, &ProjectBriefRepresentation{
			ProjectMinimalRepresentation: &ProjectMinimalRepresentation{Name: name},
			Envs:                         nameWithEnvMap[name],
		})
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
	nameWithEnvs, err := commonrepo.NewProductColl().ListProjectsInNames(opts.Names)
	if err != nil {
		return nil, nil, err
	}

	nameSet := sets.NewString()
	nameMap := make(map[string][]string)
	for _, nameWithEnv := range nameWithEnvs {
		nameSet.Insert(nameWithEnv.ProjectName)
		nameMap[nameWithEnv.ProjectName] = nameWithEnv.Envs
	}

	return nameSet, nameMap, nil
}

func getProjects(opts *ProjectListOptions) (sets.String, map[string]*templaterepo.ProjectInfo, error) {
	res, err := templaterepo.NewProductColl().ListProjectBriefs(opts.Names)
	if err != nil {
		return nil, nil, err
	}

	nameSet := sets.NewString()
	nameMap := make(map[string]*templaterepo.ProjectInfo)
	for _, r := range res {
		nameSet.Insert(r.Name)
		nameMap[r.Name] = r
	}

	return nameSet, nameMap, nil
}

func clearChartTemplate(logger *zap.SugaredLogger) {
	cts, err := commonrepo.NewChartColl().List()
	if err != nil {
		logger.Errorf("Failed to list chart templates, err: %s", err)
		return
	}

	paths := make([]string, 0)
	for _, ct := range cts {
		paths = append(paths, config.ObjectStorageChartTemplatePath(ct.Name))
	}

	err = fsservice.DeleteDirFromS3(paths, logger)
	if err != nil {
		logger.Errorf("failed to delete dir from s3, err: %s", err)
	}
}

func ClearProject(requestID string, log *zap.SugaredLogger) error {

	errList := new(multierror.Error)

	projects, err := templaterepo.NewProductColl().List()
	if err != nil {
		return errors.Wrapf(err, "failed to list projects")
	}

	// delete resources created by zadig
	for _, project := range projects {
		projectName := project.ProjectName

		// test modules need to be deleted explicitly to delete related webhooks
		if err = DeleteTestModules(projectName, requestID, log); err != nil {
			errList = multierror.Append(errList, errors.Wrapf(err, "failed to delete test modules for porject: %s", projectName))
			continue
		}

		// workflows modules need to be deleted explicitly to delete related webhooks
		if err = commonservice.DeleteWorkflows(projectName, requestID, log); err != nil {
			errList = multierror.Append(errList, errors.Wrapf(err, "failed to delete workflows for porject: %s", projectName))
			continue
		}

		// pipelines need to be deleted explicitly to delete related webhooks
		if err = commonservice.DeletePipelines(projectName, requestID, log); err != nil {
			errList = multierror.Append(errList, errors.Wrapf(err, "failed to delete pipelines for porject: %s", projectName))
			continue
		}

		// delete namespaces and other resource created by zadig
		err = DeleteProductsAsync("zadig", projectName, requestID, true, log)
		if err != nil {
			errList = multierror.Append(errors.Wrapf(err, "failed to delete product for project: %s", projectName))
			continue
		}

	}

	if errList.ErrorOrNil() != nil {
		return errList.ErrorOrNil()
	}

	// clean cache should not block clean progress
	err = service.CleanCache()
	if err != nil {
		log.Errorf("clean cache failed: %s", err)
	}

	// clear chart template cache
	clearChartTemplate(log)

	return nil
}
