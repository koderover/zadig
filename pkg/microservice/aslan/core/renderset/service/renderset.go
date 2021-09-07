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
	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

//type CodehostInfo struct {
//	CodehostID int    `bson:"codehost_id"                    json:"codehost_id"`
//	RepoName   string `bson:"repo_name"                      json:"repo_name"`
//	RepoOwner  string `bson:"repo_owner"                     json:"repo_owner"`
//	BranchName string `bson:"branch_name"                    json:"branch_name"`
//	LoadPath   string `bson:"load_path"                      json:"load_path"`
//}

type RendersetValuesArgs struct {
	RequestID      string                   `json:"request_id"`
	EnvName        string                   `json:"env_name"`
	YamlSource     string                   `json:"yaml_source"`
	YamlContent    string                   `json:"yaml_content"`
	GitConfigInfos []*template.GitConfigSet `json:"git_config_infos"`
	OverrideValues []*template.KVPair       `json:"override_values"`
	UpdateBy       string                   `json:"update_by,omitempty"`
}

func ListChartValues(productName, serviceName string, args *RendersetValuesArgs, log *zap.SugaredLogger) (*template.RenderChart, error) {

	renderSetName := commonservice.GetProductEnvNamespace(args.EnvName, productName)

	opt := &commonrepo.RenderSetFindOption{
		Name: renderSetName,
	}
	rendersetObj, err := commonrepo.NewRenderSetColl().Find(opt)
	if err != nil || rendersetObj == nil {
		return nil, err
	}

	for _, singleChart := range rendersetObj.ChartInfos {
		if singleChart.ServiceName == serviceName {
			return singleChart, nil
		}
	}

	return nil, nil
}

func CreateOrUpdateChartValues(productName, serviceName string, args *RendersetValuesArgs, log *zap.SugaredLogger) error {

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

	yamlContent := ""
	if args.YamlSource == "free_edit" {
		if args.YamlContent == "" {
			return e.ErrCreateRenderSet.AddDesc("yaml content can't be empty")
		}
		yamlContent = args.YamlContent
	} else if args.YamlSource == "default" {
		yamlContent = serviceObj.HelmChart.ValuesYaml
	} else {
		return e.ErrCreateRenderSet.AddDesc("git import yaml not supported yet")
	}

	renderSetName := commonservice.GetProductEnvNamespace(args.EnvName, productName)

	rendersetObj := &commonmodels.RenderSet{
		Name:        renderSetName,
		Revision:    0,
		EnvName:     args.EnvName,
		ProductTmpl: productName,
		UpdateBy:    args.UpdateBy,
	}

	opt := &commonrepo.RenderSetFindOption{Name: renderSetName}
	curRenderset, err := commonrepo.NewRenderSetColl().Find(opt)
	if err != nil && err != mongo.ErrNoDocuments {
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}

	if curRenderset == nil {
		curRenderset = &commonmodels.RenderSet{
			Name:        renderSetName,
			Revision:    0,
			EnvName:     args.EnvName,
			ProductTmpl: productName,
			UpdateBy:    args.UpdateBy,
			IsDefault:   false,
		}
	}

	//update or insert service values.yaml
	findService := false
	for _, singleChart := range curRenderset.ChartInfos {
		if singleChart.ServiceName != serviceName {
			continue
		}
		findService = true
		singleChart.ValuesYaml = yamlContent
		singleChart.YamlSource = args.YamlSource
		singleChart.OverrideValues = args.OverrideValues
		if args.YamlSource == "git_repo" {
			singleChart.GitConfigList = args.GitConfigInfos
		} else {
			singleChart.GitConfigList = nil
		}
		break
	}
	if !findService {
		charInfo := &template.RenderChart{
			ServiceName:    serviceName,
			ChartVersion:   serviceObj.HelmChart.Version,
			ValuesYaml:     yamlContent,
			YamlSource:     args.YamlSource,
			OverrideValues: args.OverrideValues,
		}
		if args.YamlSource == "git_repo" {
			charInfo.GitConfigList = args.GitConfigInfos
		}
		curRenderset.ChartInfos = append(curRenderset.ChartInfos, charInfo)
	}

	//increase renderset revision
	if err := ensureHelmRenderSetArgs(rendersetObj); err != nil {
		log.Error(err)
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}

	err = commonservice.CreateHelmRenderSet(
		curRenderset,
		log,
	)

	if err != nil {
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}

	//when environment existed, need to update related services
	err = service.ApplyHelmProductRenderset(productName, args.EnvName, args.UpdateBy, args.RequestID, curRenderset, log)
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
