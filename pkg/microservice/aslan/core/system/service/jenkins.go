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
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/tool/crypto"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/types"
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

func CreateJenkinsIntegration(args *commonmodels.JenkinsIntegration, log *zap.SugaredLogger) error {
	if err := commonrepo.NewJenkinsIntegrationColl().Create(args); err != nil {
		log.Errorf("CreateJenkinsIntegration err:%v", err)
		return e.ErrCreateJenkinsIntegration.AddErr(err)
	}
	return nil
}

func ListJenkinsIntegration(encryptedKey string, log *zap.SugaredLogger) ([]*commonmodels.JenkinsIntegration, error) {
	jenkinsIntegrations, err := commonrepo.NewJenkinsIntegrationColl().List()
	if err != nil {
		log.Errorf("ListJenkinsIntegration err:%v", err)
		return []*commonmodels.JenkinsIntegration{}, e.ErrListJenkinsIntegration.AddErr(err)
	}

	if len(encryptedKey) == 0 {
		return jenkinsIntegrations, nil
	}

	aesKey, err := service.GetAesKeyFromEncryptedKey(encryptedKey, log)
	if err != nil {
		log.Errorf("ListJenkinsIntegration GetAesKeyFromEncryptedKey err:%v", err)
		return nil, err
	}
	for _, integration := range jenkinsIntegrations {
		integration.Password, err = crypto.AesEncryptByKey(integration.Password, aesKey.PlainText)
		if err != nil {
			log.Errorf("ListJenkinsIntegration AesEncryptByKey err:%v", err)
			return nil, err
		}
	}
	return jenkinsIntegrations, nil
}

func UpdateJenkinsIntegration(ID string, args *commonmodels.JenkinsIntegration, log *zap.SugaredLogger) error {
	if err := commonrepo.NewJenkinsIntegrationColl().Update(ID, args); err != nil {
		log.Errorf("UpdateJenkinsIntegration err:%v", err)
		return e.ErrUpdateJenkinsIntegration.AddErr(err)
	}
	return nil
}

func DeleteJenkinsIntegration(ID string, log *zap.SugaredLogger) error {
	if err := commonrepo.NewJenkinsIntegrationColl().Delete(ID); err != nil {
		log.Errorf("DeleteJenkinsIntegration err:%v", err)
		return e.ErrDeleteJenkinsIntegration.AddErr(err)
	}
	return nil
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
	jenkinsIntegration, err := commonrepo.NewJenkinsIntegrationColl().Get(id)
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
	jenkinsClient, ctx, err := getJenkinsClient(id, log)
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

func ListJobBuildArgs(id, jobName string, log *zap.SugaredLogger) ([]*JenkinsBuildArgs, error) {
	jenkinsClient, ctx, err := getJenkinsClient(id, log)
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
		arg := &JenkinsBuildArgs{
			Name:  paramDefinition.DefaultParameterValue.Name,
			Value: paramDefinition.DefaultParameterValue.Value,
			Type:  paramDefinition.Type,
		}
		if paramDefinition.Type == "ChoiceParameterDefinition" {
			arg.Type = string(types.Choice)
		} else {
			arg.Type = string(types.Str)
		}
		jenkinsBuildArgsResp = append(jenkinsBuildArgsResp, arg)
	}
	return jenkinsBuildArgsResp, nil
}
