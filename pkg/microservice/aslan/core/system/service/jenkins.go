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

	"github.com/koderover/gojenkins"
	"go.uber.org/zap"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	jenkinsclient "github.com/koderover/zadig/v2/pkg/tool/jenkins"
	"github.com/koderover/zadig/v2/pkg/types"
)

type JenkinsArgs struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type JenkinsBuildArgs struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}

func TestJenkinsConnection(args *JenkinsArgs, log *zap.SugaredLogger) error {
	ctx := context.Background()
	_, err := gojenkins.CreateJenkins(nil, args.URL, args.Username, args.Password).Init(ctx)
	if err != nil {
		log.Errorf("TestJenkinsConnection err:%v", err)
		return e.ErrTestJenkinsConnection.AddErr(err)
	}
	return nil
}

func getJenkinsClient(id string, log *zap.SugaredLogger) (*gojenkins.Jenkins, context.Context, error) {
	ctx := context.Background()
	jenkinsIntegration, err := commonrepo.NewCICDToolColl().Get(id)
	if err != nil {
		return nil, ctx, fmt.Errorf("未找到jenkins集成数据")
	}

	jenkinsClient, err := gojenkins.CreateJenkins(nil, jenkinsIntegration.URL, jenkinsIntegration.Username, jenkinsIntegration.Password).Init(ctx)
	if err != nil {
		return nil, ctx, err
	}
	return jenkinsClient, ctx, nil
}

func ListJobNames(id string, log *zap.SugaredLogger) ([]string, error) {
	jenkinsIntegration, err := commonrepo.NewCICDToolColl().Get(id)
	if err != nil {
		return []string{}, e.ErrListJobNames.AddErr(err)
	}

	jenkinsClient := jenkinsclient.NewClient(jenkinsIntegration.URL, jenkinsIntegration.Username, jenkinsIntegration.Password)
	jobNames, err := jenkinsClient.ListJobNames()
	if err != nil {
		return []string{}, e.ErrListJobNames.AddErr(err)
	}
	return jobNames, nil
}

func ListJobBuildArgs(id, jobName string, log *zap.SugaredLogger) ([]*JenkinsBuildArgs, error) {
	jenkinsIntegration, err := commonrepo.NewCICDToolColl().Get(id)
	if err != nil {
		return []*JenkinsBuildArgs{}, e.ErrListJobBuildArgs.AddErr(err)
	}

	jenkinsClient := jenkinsclient.NewClient(jenkinsIntegration.URL, jenkinsIntegration.Username, jenkinsIntegration.Password)
	jenkinsJob, err := jenkinsClient.GetJob(jobName)
	if err != nil {
		return []*JenkinsBuildArgs{}, e.ErrListJobBuildArgs.AddErr(err)
	}
	jenkinsBuildArgsResp := make([]*JenkinsBuildArgs, 0)
	for _, paramDefinition := range jenkinsJob.GetParameters() {
		arg := &JenkinsBuildArgs{
			Name:  paramDefinition.Name,
			Value: paramDefinition.DefaultParameterValue.Value,
			Type:  string(paramDefinition.Type),
		}
		if paramDefinition.Type == jenkinsclient.Choice {
			arg.Type = string(types.Choice)
		} else {
			arg.Type = string(types.Str)
		}
		jenkinsBuildArgsResp = append(jenkinsBuildArgsResp, arg)
	}
	return jenkinsBuildArgsResp, nil
}
