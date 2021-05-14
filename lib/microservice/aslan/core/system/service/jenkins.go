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
	"context"
	"fmt"

	"github.com/bndr/gojenkins"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

type JenkinsArgs struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type JenkinsBuildArgs struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

func CreateJenkinsIntegration(args *commonmodels.JenkinsIntegration, log *xlog.Logger) error {
	if err := commonrepo.NewJenkinsIntegrationColl().Create(args); err != nil {
		log.Errorf("CreateJenkinsIntegration err:%v", err)
		return e.ErrCreateJenkinsIntegration.AddErr(err)
	}
	return nil
}

func ListJenkinsIntegration(log *xlog.Logger) ([]*commonmodels.JenkinsIntegration, error) {
	jenkinsIntegrations, err := commonrepo.NewJenkinsIntegrationColl().List()
	if err != nil {
		log.Errorf("ListJenkinsIntegration err:%v", err)
		return []*commonmodels.JenkinsIntegration{}, e.ErrListJenkinsIntegration.AddErr(err)
	}

	return jenkinsIntegrations, nil
}

func UpdateJenkinsIntegration(ID string, args *commonmodels.JenkinsIntegration, log *xlog.Logger) error {
	if err := commonrepo.NewJenkinsIntegrationColl().Update(ID, args); err != nil {
		log.Errorf("UpdateJenkinsIntegration err:%v", err)
		return e.ErrUpdateJenkinsIntegration.AddErr(err)
	}
	return nil
}

func DeleteJenkinsIntegration(ID string, log *xlog.Logger) error {
	if err := commonrepo.NewJenkinsIntegrationColl().Delete(ID); err != nil {
		log.Errorf("DeleteJenkinsIntegration err:%v", err)
		return e.ErrDeleteJenkinsIntegration.AddErr(err)
	}
	return nil
}

func TestJenkinsConnection(args *JenkinsArgs, log *xlog.Logger) error {
	ctx := context.Background()
	_, err := gojenkins.CreateJenkins(nil, args.URL, args.Username, args.Password).Init(ctx)
	if err != nil {
		log.Errorf("TestJenkinsConnection err:%v", err)
		return e.ErrTestJenkinsConnection.AddErr(err)
	}
	return nil
}

func getJenkinsClient(log *xlog.Logger) (*gojenkins.Jenkins, context.Context, error) {
	ctx := context.Background()
	jenkinsIntegrations, err := ListJenkinsIntegration(log)
	if err != nil {
		return nil, ctx, err
	}
	if len(jenkinsIntegrations) == 0 {
		return nil, ctx, fmt.Errorf("未找到jenkins集成数据")
	}
	jenkinsClient, err := gojenkins.CreateJenkins(nil, jenkinsIntegrations[0].URL, jenkinsIntegrations[0].Username, jenkinsIntegrations[0].Password).Init(ctx)
	if err != nil {
		return nil, ctx, err
	}
	return jenkinsClient, ctx, nil
}

func ListJobNames(log *xlog.Logger) ([]string, error) {
	jenkinsClient, ctx, err := getJenkinsClient(log)
	if err != nil {
		return []string{}, e.ErrListJobNames.AddErr(err)
	}
	innerJobs, err := jenkinsClient.GetAllJobNames(ctx)
	if err != nil {
		return []string{}, e.ErrListJobNames.AddErr(err)
	}
	jobNames := make([]string, 0)
	for _, innerJob := range innerJobs {
		jobNames = append(jobNames, innerJob.Name)
	}
	return jobNames, nil
}

func ListJobBuildArgs(jobName string, log *xlog.Logger) ([]*JenkinsBuildArgs, error) {
	jenkinsClient, ctx, err := getJenkinsClient(log)
	if err != nil {
		return []*JenkinsBuildArgs{}, e.ErrListJobBuildArgs.AddErr(err)
	}
	jenkinsJob, err := jenkinsClient.GetJob(ctx, jobName)
	if err != nil {
		return []*JenkinsBuildArgs{}, e.ErrListJobBuildArgs.AddErr(err)
	}
	paramDefinitions, err := jenkinsJob.GetParameters(ctx)
	if err != nil {
		return []*JenkinsBuildArgs{}, e.ErrListJobBuildArgs.AddErr(err)
	}
	jenkinsBuildArgsResp := make([]*JenkinsBuildArgs, 0)
	for _, paramDefinition := range paramDefinitions {
		jenkinsBuildArgsResp = append(jenkinsBuildArgsResp, &JenkinsBuildArgs{
			Name:  paramDefinition.DefaultParameterValue.Name,
			Value: paramDefinition.DefaultParameterValue.Value,
		})
	}
	return jenkinsBuildArgsResp, nil
}
