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
	"strings"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	yaml2 "github.com/koderover/zadig/pkg/util/yaml"
)

func GetRenderCharts(productName, envName, serviceName string, log *zap.SugaredLogger) ([]*commonservice.RenderChartArg, error) {

	renderSetName := commonservice.GetProductEnvNamespace(envName, productName)

	opt := &commonrepo.RenderSetFindOption{
		Name: renderSetName,
	}
	rendersetObj, existed, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil {
		return nil, err
	}

	ret := make([]*commonservice.RenderChartArg, 0)

	if !existed {
		return ret, nil
	}

	serverList := strings.Split(serviceName, ",")
	serviceMap := map[string]int{}
	for _, singleService := range serverList {
		serviceMap[singleService] = 1
	}

	matchedRenderChartModels := make([]*template.RenderChart, 0)
	if len(serviceMap) == 0 {
		matchedRenderChartModels = rendersetObj.ChartInfos
	} else {
		for _, singleChart := range rendersetObj.ChartInfos {
			if _, ok := serviceMap[singleChart.ServiceName]; !ok {
				continue
			}
			matchedRenderChartModels = append(matchedRenderChartModels, singleChart)
		}
	}

	for _, singleChart := range matchedRenderChartModels {
		rcaObj := new(commonservice.RenderChartArg)
		rcaObj.LoadFromRenderChartModel(singleChart)
		ret = append(ret, rcaObj)
	}
	return ret, nil
}

// validate yaml content
func validateYamlContent(yamlContent string) error {
	tMap := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(yamlContent), &tMap); err != nil {
		if err != nil {
			return errors.New("yaml content illegal")
		}
	}
	return nil
}

func generateValuesYaml(service *commonmodels.Service, args *commonservice.RenderChartArg, log *zap.SugaredLogger) (string, error) {
	if args.YamlSource == setting.ValuesYamlSourceFreeEdit {
		return args.ValuesYAML, validateYamlContent(args.ValuesYAML)
	} else if args.YamlSource == setting.ValuesYamlSourceDefault {
		return service.HelmChart.ValuesYaml, nil
	} else if args.YamlSource == setting.ValuesYamlSourceGitRepo {
		if args.GitRepoConfig == nil {
			return "", nil
		}

		var (
			allValues      []byte
			fileContentMap sync.Map
			wg             sync.WaitGroup
			err            error
		)

		for index, filePath := range args.GitRepoConfig.ValuesPaths {
			wg.Add(1)
			go func(index int, path string) {
				defer wg.Done()
				fileContent, err1 := fsservice.DownloadFileFromSource(
					&fsservice.DownloadFromSourceArgs{
						CodehostID: args.GitRepoConfig.CodehostID,
						Owner:      args.GitRepoConfig.Owner,
						Repo:       args.GitRepoConfig.Repo,
						Path:       path,
						Branch:     args.GitRepoConfig.Branch,
					})
				if err1 != nil {
					err = errors.Errorf("fail to download file from git, err: %s, path: %s, repo: %v", err1.Error(), path, *args.GitRepoConfig)
					return
				}
				fileContentMap.Store(index, fileContent)
			}(index, filePath)
		}
		wg.Wait()

		if err != nil {
			return "", err
		}

		allValueYamls := make([][]byte, len(args.GitRepoConfig.ValuesPaths), len(args.GitRepoConfig.ValuesPaths))
		for i := 0; i < len(args.GitRepoConfig.ValuesPaths); i++ {
			contentObj, _ := fileContentMap.Load(i)
			allValueYamls[i] = contentObj.([]byte)
		}
		allValues, err = yaml2.Merge(allValueYamls)
		if err != nil {
			return "", errors.Errorf("failed to merge yaml files, repo: %v", *args.GitRepoConfig)
		}
		return string(allValues), nil
	}
	return "", nil
}

func CreateOrUpdateChartValues(productName, envName string, args *commonservice.RenderChartArg, userName, requestID string, log *zap.SugaredLogger) error {

	serviceName := args.ServiceName

	serviceOpt := &commonrepo.ServiceFindOption{
		ProductName: productName,
		ServiceName: serviceName,
		Type: setting.HelmDeployType,
	}
	serviceObj, err := commonrepo.NewServiceColl().Find(serviceOpt)
	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}
	if serviceObj == nil {
		return e.ErrCreateRenderSet.AddDesc("service not found")
	}

	if serviceObj.HelmChart == nil {
		return e.ErrCreateRenderSet.AddDesc("missing helm chart info")
	}

	yamlContent, err := generateValuesYaml(serviceObj, args, log)
	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}

	if yamlContent == "" {
		return e.ErrCreateRenderSet.AddDesc("empty yaml content")
	}

	renderSetName := commonservice.GetProductEnvNamespace(envName, productName)

	opt := &commonrepo.RenderSetFindOption{Name: renderSetName}
	curRenderset,_, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}

	if curRenderset == nil {
		curRenderset = &commonmodels.RenderSet{
			Name:        renderSetName,
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
		args.FillRenderChartModel(singleChart, singleChart.ChartVersion)
		targetChartInfo = singleChart
		break
	}
	if targetChartInfo == nil {
		targetChartInfo = new(template.RenderChart)
		args.FillRenderChartModel(targetChartInfo, serviceObj.HelmChart.Version)
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
