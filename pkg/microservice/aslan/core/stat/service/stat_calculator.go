/*
Copyright 2023 The KodeRover Authors.

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
	"strings"

	"github.com/Knetic/govaluate"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/util"
)

type StatCalculator interface {
	GetWeightedScore(score float64) (float64, error)
	GetFact(startTime int64, endTime int64, projectKey string) (float64, error)
}

func CreateCalculatorFromConfig(cfg *StatDashboardConfig) (StatCalculator, error) {
	// if the data source of the calculator is from API, then we find the external system and return a generalCalculator
	if cfg.Source == "api" {
		externalSystem, err := commonrepo.NewExternalSystemColl().GetByID(cfg.APIConfig.ExternalSystemId)
		if err != nil {
			return nil, err
		}
		return &GeneralCalculator{
			Host:     externalSystem.Server,
			Path:     cfg.APIConfig.ApiPath,
			Queries:  cfg.APIConfig.Queries,
			Headers:  externalSystem.Headers,
			Weight:   cfg.Weight,
			Function: cfg.Function,
		}, nil
	}
	switch cfg.ID {
	case config.DashboardDataTypeTestPassRate:
		return &TestPassRateCalculator{
			Weight:   cfg.Weight,
			Function: cfg.Function,
		}, nil
	case config.DashboardDataTypeTestAverageDuration:
		return &TestAverageDurationCalculator{
			Weight:   cfg.Weight,
			Function: cfg.Function,
		}, nil
	case config.DashboardDataTypeBuildSuccessRate:
		return &BuildSuccessRateCalculator{
			Weight:   cfg.Weight,
			Function: cfg.Function,
		}, nil
	case config.DashboardDataTypeBuildAverageDuration:
		return &BuildAverageDurationCalculator{
			Weight:   cfg.Weight,
			Function: cfg.Function,
		}, nil
	case config.DashboardDataTypeBuildFrequency:
		return &BuildFrequencyCalculator{
			Weight:   cfg.Weight,
			Function: cfg.Function,
		}, nil
	case config.DashboardDataTypeDeploySuccessRate:
		return &DeploySuccessRateCalculator{
			Weight:   cfg.Weight,
			Function: cfg.Function,
		}, nil
	case config.DashboardDataTypeDeployAverageDuration:
		return &DeployAverageDurationCalculator{
			Weight:   cfg.Weight,
			Function: cfg.Function,
		}, nil
	case config.DashboardDataTypeDeployFrequency:
		return &DeployFrequencyCalculator{
			Weight:   cfg.Weight,
			Function: cfg.Function,
		}, nil
	case config.DashboardDataTypeReleaseSuccessRate:
		return &ReleaseFrequencyCalculator{
			Weight:   cfg.Weight,
			Function: cfg.Function,
		}, nil
	case config.DashboardDataTypeReleaseAverageDuration:
		return &ReleaseFrequencyCalculator{
			Weight:   cfg.Weight,
			Function: cfg.Function,
		}, nil
	case config.DashboardDataTypeReleaseFrequency:
		return &ReleaseFrequencyCalculator{
			Weight:   cfg.Weight,
			Function: cfg.Function,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported config id: %s", cfg.ID)
	}
}

// GeneralCalculator gets the facts from the given API from the APIConfig
// and calculate the score based on Function and Weight
type GeneralCalculator struct {
	Host     string
	Path     string
	Queries  []*util.KeyValue
	Headers  []*util.KeyValue
	Weight   int64
	Function string
}

func (c *GeneralCalculator) GetWeightedScore(fact float64) (float64, error) {
	return calculateWeightedScore(fact, c.Function, c.Weight)
}

func (c *GeneralCalculator) GetFact(startTime, endTime int64, project string) (float64, error) {
	var fact float64
	host := strings.TrimSuffix(c.Host, "/")
	url := fmt.Sprintf("%s/%s", host, c.Path)
	queryMap := make(map[string]string)
	for _, query := range c.Queries {
		queryMap[query.Key] = query.Value.(string)
	}
	headerMap := make(map[string]string)
	for _, header := range c.Headers {
		headerMap[header.Key] = header.Value.(string)
	}
	_, err := httpclient.Get(url, httpclient.SetQueryParams(queryMap), httpclient.SetHeaders(headerMap), httpclient.SetResult(&fact))
	if err != nil {
		return 0, err
	}
	return fact, nil
}

// TestPassRateCalculator is used when the data ID is "test_pass_rate" and the data source is "zadig"
type TestPassRateCalculator struct {
	Weight   int64
	Function string
}

func (c *TestPassRateCalculator) GetFact(startTime, endTime int64, project string) (float64, error) {
	testJobList, err := commonrepo.NewJobInfoColl().GetTestJobs(startTime, endTime, project)
	if err != nil {
		return 0, err
	}
	totalCounter := len(testJobList)
	passCounter := 0
	for _, job := range testJobList {
		if job.Status == string(config.StatusPassed) {
			passCounter++
		}
	}
	return float64(passCounter) * 100 / float64(totalCounter), nil
}

func (c *TestPassRateCalculator) GetWeightedScore(fact float64) (float64, error) {
	return calculateWeightedScore(fact, c.Function, c.Weight)
}

type TestAverageDurationCalculator struct {
	Weight   int64
	Function string
}

func (c *TestAverageDurationCalculator) GetFact(startTime, endTime int64, project string) (float64, error) {
	testJobList, err := commonrepo.NewJobInfoColl().GetTestJobs(startTime, endTime, project)
	if err != nil {
		return 0, err
	}
	totalCounter := len(testJobList)
	var totalTimesTaken int64 = 0
	for _, job := range testJobList {
		totalTimesTaken += job.Duration
	}
	return float64(totalTimesTaken) / float64(totalCounter), nil
}

func (c *TestAverageDurationCalculator) GetWeightedScore(fact float64) (float64, error) {
	return calculateWeightedScore(fact, c.Function, c.Weight)
}

func calculateWeightedScore(fact float64, function string, weight int64) (float64, error) {
	expression, err := govaluate.NewEvaluableExpression(function)
	if err != nil {
		return 0, err
	}
	variables := map[string]interface{}{
		"x": fact,
	}
	result, err := expression.Evaluate(variables)
	if err != nil {
		return 0, err
	}
	scoreWithoutWeight, ok := result.(float64)
	if !ok {
		return 0, fmt.Errorf("failed to convert result to float64")
	}
	return scoreWithoutWeight * float64(weight) / 100, nil
}

type BuildSuccessRateCalculator struct {
	Weight   int64
	Function string
}

func (c *BuildSuccessRateCalculator) GetFact(startTime, endTime int64, project string) (float64, error) {
	buildJobList, err := commonrepo.NewJobInfoColl().GetBuildJobs(startTime, endTime, project)
	if err != nil {
		return 0, err
	}
	totalCounter := len(buildJobList)
	passCounter := 0
	for _, job := range buildJobList {
		if job.Status == string(config.StatusPassed) {
			passCounter++
		}
	}
	return float64(passCounter) * 100 / float64(totalCounter), nil
}

func (c *BuildSuccessRateCalculator) GetWeightedScore(fact float64) (float64, error) {
	return calculateWeightedScore(fact, c.Function, c.Weight)
}

type BuildAverageDurationCalculator struct {
	Weight   int64
	Function string
}

func (c *BuildAverageDurationCalculator) GetFact(startTime, endTime int64, project string) (float64, error) {
	buildJobList, err := commonrepo.NewJobInfoColl().GetBuildJobs(startTime, endTime, project)
	if err != nil {
		return 0, err
	}
	totalCounter := len(buildJobList)
	var totalTimesTaken int64 = 0
	for _, job := range buildJobList {
		totalTimesTaken += job.Duration
	}
	return float64(totalTimesTaken) / float64(totalCounter), nil
}

func (c *BuildAverageDurationCalculator) GetWeightedScore(fact float64) (float64, error) {
	return calculateWeightedScore(fact, c.Function, c.Weight)
}

type BuildFrequencyCalculator struct {
	Weight   int64
	Function string
}

func (c *BuildFrequencyCalculator) GetFact(startTime, endTime int64, project string) (float64, error) {
	buildJobList, err := commonrepo.NewJobInfoColl().GetBuildJobs(startTime, endTime, project)
	if err != nil {
		return 0, err
	}

	daysBetween := int(endTime-startTime) / 86400
	totalCounter := len(buildJobList)

	return float64(totalCounter) * 7 / float64(daysBetween), nil
}

func (c *BuildFrequencyCalculator) GetWeightedScore(fact float64) (float64, error) {
	return calculateWeightedScore(fact, c.Function, c.Weight)
}

type DeploySuccessRateCalculator struct {
	Weight   int64
	Function string
}

func (c *DeploySuccessRateCalculator) GetFact(startTime, endTime int64, project string) (float64, error) {
	deployJobList, err := commonrepo.NewJobInfoColl().GetDeployJobs(startTime, endTime, project)
	if err != nil {
		return 0, err
	}
	totalCounter := len(deployJobList)
	passCounter := 0
	for _, job := range deployJobList {
		if job.Status == string(config.StatusPassed) {
			passCounter++
		}
	}
	return float64(passCounter) * 100 / float64(totalCounter), nil
}

func (c *DeploySuccessRateCalculator) GetWeightedScore(fact float64) (float64, error) {
	return calculateWeightedScore(fact, c.Function, c.Weight)
}

type DeployAverageDurationCalculator struct {
	Weight   int64
	Function string
}

func (c *DeployAverageDurationCalculator) GetFact(startTime, endTime int64, project string) (float64, error) {
	deployJobList, err := commonrepo.NewJobInfoColl().GetDeployJobs(startTime, endTime, project)
	if err != nil {
		return 0, err
	}
	totalCounter := len(deployJobList)
	var totalTimesTaken int64 = 0
	for _, job := range deployJobList {
		totalTimesTaken += job.Duration
	}
	return float64(totalTimesTaken) / float64(totalCounter), nil
}

func (c *DeployAverageDurationCalculator) GetWeightedScore(fact float64) (float64, error) {
	return calculateWeightedScore(fact, c.Function, c.Weight)
}

type DeployFrequencyCalculator struct {
	Weight   int64
	Function string
}

func (c *DeployFrequencyCalculator) GetFact(startTime, endTime int64, project string) (float64, error) {
	deployJobList, err := commonrepo.NewJobInfoColl().GetDeployJobs(startTime, endTime, project)
	if err != nil {
		return 0, err
	}

	daysBetween := int(endTime-startTime) / 86400
	totalCounter := len(deployJobList)

	return float64(totalCounter) * 7 / float64(daysBetween), nil
}

func (c *DeployFrequencyCalculator) GetWeightedScore(fact float64) (float64, error) {
	return calculateWeightedScore(fact, c.Function, c.Weight)
}

type ReleaseSuccessRateCalculator struct {
	Weight   int64
	Function string
}

func (c *ReleaseSuccessRateCalculator) GetFact(startTime, endTime int64, project string) (float64, error) {
	releaseJobList, err := commonrepo.NewJobInfoColl().GetProductionDeployJobs(startTime, endTime, project)
	if err != nil {
		return 0, err
	}
	totalCounter := len(releaseJobList)
	passCounter := 0
	for _, job := range releaseJobList {
		if job.Status == string(config.StatusPassed) {
			passCounter++
		}
	}
	return float64(passCounter) * 100 / float64(totalCounter), nil
}

func (c *ReleaseSuccessRateCalculator) GetWeightedScore(fact float64) (float64, error) {
	return calculateWeightedScore(fact, c.Function, c.Weight)
}

type ReleaseAverageDurationCalculator struct {
	Weight   int64
	Function string
}

func (c *ReleaseAverageDurationCalculator) GetFact(startTime, endTime int64, project string) (float64, error) {
	releaseJobList, err := commonrepo.NewJobInfoColl().GetProductionDeployJobs(startTime, endTime, project)
	if err != nil {
		return 0, err
	}
	totalCounter := len(releaseJobList)
	var totalTimesTaken int64 = 0
	for _, job := range releaseJobList {
		totalTimesTaken += job.Duration
	}
	return float64(totalTimesTaken) / float64(totalCounter), nil
}

func (c *ReleaseAverageDurationCalculator) GetWeightedScore(fact float64) (float64, error) {
	return calculateWeightedScore(fact, c.Function, c.Weight)
}

type ReleaseFrequencyCalculator struct {
	Weight   int64
	Function string
}

func (c *ReleaseFrequencyCalculator) GetFact(startTime, endTime int64, project string) (float64, error) {
	releaseJobList, err := commonrepo.NewJobInfoColl().GetProductionDeployJobs(startTime, endTime, project)
	if err != nil {
		return 0, err
	}

	daysBetween := int(endTime-startTime) / 86400
	totalCounter := len(releaseJobList)

	return float64(totalCounter) * 7 / float64(daysBetween), nil
}

func (c *ReleaseFrequencyCalculator) GetWeightedScore(fact float64) (float64, error) {
	return calculateWeightedScore(fact, c.Function, c.Weight)
}
