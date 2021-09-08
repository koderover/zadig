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

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	yaml2 "github.com/koderover/zadig/pkg/util/yaml"
)

type RendersetValuesArgs struct {
	RenderCharts []*template.RenderChart `json:"render_charts"`
}

func ListChartRenders(productName, envName, serviceName string, log *zap.SugaredLogger) ([]*template.RenderChart, error) {

	renderSetName := commonservice.GetProductEnvNamespace(envName, productName)

	opt := &commonrepo.RenderSetFindOption{
		Name: renderSetName,
	}
	rendersetObj, err := commonrepo.NewRenderSetColl().Find(opt)
	if err != nil || rendersetObj == nil {
		return nil, err
	}

	if serviceName == "" {
		return rendersetObj.ChartInfos, nil
	}

	for _, singleChart := range rendersetObj.ChartInfos {
		if singleChart.ServiceName == serviceName {
			return []*template.RenderChart{singleChart}, nil
		}
	}
	return nil, nil
}

func generateValuesValue(service *commonmodels.Service, args *template.RenderChart, log *zap.SugaredLogger) (string, error) {
	if args.YamlSource == "free_edit" {
		return args.ValuesYaml, nil
	} else if args.YamlSource == "default" {
		if service.HelmChart != nil {
			return service.HelmChart.ValuesYaml, nil
		}
	} else if args.YamlSource == "git_repo" {
		// TODO need optimize, use parallel execution
		var allValues []byte
		var err error
		for _, repoArgs := range args.GitConfigList {
			var valuesByRepo []byte
			for _, filePath := range repoArgs.Paths {
				fileContent, err := fsservice.DownloadFileFromSource(
					&fsservice.DownloadFromSourceArgs{CodehostID: repoArgs.CodehostID, Owner: repoArgs.Owner, Repo: repoArgs.Repo, Path: filePath, Branch: repoArgs.Branch})
				if err != nil {
					return "", errors.Errorf("fail to download file from git, path: %s, repo: %v", filePath, *repoArgs)
				}
				valuesByRepo, err = yaml2.MergeBoth(valuesByRepo, fileContent)
				if err != nil {
					return "", errors.Errorf("fail to merge file, path: %s, repo: %v", filePath, *repoArgs)
				}
			}
			allValues, err = yaml2.MergeBoth(allValues, valuesByRepo)
			if err != nil {
				return "", errors.Errorf("fail to merge file: %v", *repoArgs)
			}
		}
		return string(allValues), nil
	}
	return "", nil
}

func CreateOrUpdateChartValues(productName, envName string, args *template.RenderChart, userName, requestID string, log *zap.SugaredLogger) error {

	serviceName := args.ServiceName

	serviceOpt := &commonrepo.ServiceFindOption{
		ProductName: productName,
		ServiceName: serviceName,
		Revision:    0,
	}
	serviceObj, err := commonrepo.NewServiceColl().Find(serviceOpt)
	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}
	if serviceObj == nil {
		return e.ErrCreateRenderSet.AddDesc("service not found")
	}
	if serviceObj.Type != setting.HelmDeployType {
		return e.ErrCreateRenderSet.AddDesc("invalid service type")
	}

	yamlContent, err := generateValuesValue(serviceObj, args, log)
	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}

	if yamlContent == "" {
		return e.ErrCreateRenderSet.AddDesc("empty yaml content")
	}

	renderSetName := commonservice.GetProductEnvNamespace(envName, productName)

	opt := &commonrepo.RenderSetFindOption{Name: renderSetName}
	curRenderset, err := commonrepo.NewRenderSetColl().Find(opt)
	if err != nil && err != mongo.ErrNoDocuments {
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}

	if curRenderset == nil {
		curRenderset = &commonmodels.RenderSet{
			Name:        renderSetName,
			Revision:    0,
			EnvName:     envName,
			ProductTmpl: productName,
			UpdateBy:    userName,
			IsDefault:   false,
		}
	}

	//update or insert service values.yaml
	var targetChartInfo *template.RenderChart
	for _, singleChart := range curRenderset.ChartInfos {
		if singleChart.ServiceName != serviceName {
			continue
		}
		singleChart.ValuesYaml = yamlContent
		singleChart.YamlSource = args.YamlSource
		singleChart.OverrideValues = args.OverrideValues
		if args.YamlSource == "git_repo" {
			singleChart.GitConfigSetList = &template.GitConfigSetList{
				GitConfigList: args.GitConfigSetList.GitConfigList,
			}
		} else {
			singleChart.GitConfigSetList = nil
		}
		targetChartInfo = singleChart
		break
	}
	if targetChartInfo == nil {
		targetChartInfo = &template.RenderChart{
			ServiceName:    serviceName,
			ChartVersion:   serviceObj.HelmChart.Version,
			ValuesYaml:     yamlContent,
			YamlSource:     args.YamlSource,
			OverrideValues: args.OverrideValues,
		}
		if args.YamlSource == "git_repo" {
			targetChartInfo.GitConfigList = args.GitConfigList
		}
		curRenderset.ChartInfos = append(curRenderset.ChartInfos, targetChartInfo)
	}

	//create new renderset with increased revision
	err = commonservice.CreateHelmRenderSet(
		curRenderset,
		log,
	)

	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}

	//when environment existed, need to update related services
	err = service.ApplyHelmProductRenderset(productName, envName, userName, requestID, curRenderset, targetChartInfo, log)
	if err != mongo.ErrNoDocuments {
		return err
	}
	return err
}

func ensureHelmRenderSetArgs(args *commonmodels.RenderSet) error {
	if args == nil {
		return errors.New("nil RenderSet")
	}

	if len(args.Name) == 0 {
		return errors.New("empty render set name")
	}
	// 设置新的版本号
	rev, err := commonrepo.NewCounterColl().GetNextSeq("renderset:" + args.Name)
	if err != nil {
		return fmt.Errorf("get next render set revision error: %v", err)
	}

	args.Revision = rev
	return nil
}
