/*
Copyright 2022 The KodeRover Authors.

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

package openapi

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

func ToScanningAdvancedSetting(arg *types.OpenAPIAdvancedSetting) (*models.ScanningAdvancedSetting, error) {
	cluster, err := commonrepo.NewK8SClusterColl().FindByName(arg.ClusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to find cluster of name: %s, the error is: %s", arg.ClusterName, err)
	}

	strategy := &models.ScheduleStrategy{}
	if cluster.AdvancedConfig != nil {
		for _, s := range cluster.AdvancedConfig.ScheduleStrategy {
			if s.StrategyName == arg.StrategyName {
				strategy = s
				break
			}
		}
	}

	// scanninghooks, err := ToScanningHookCtl(arg.Webhooks)
	// if err != nil {
	// 	return nil, err
	// }

	return &models.ScanningAdvancedSetting{
		ClusterID:  cluster.ID.Hex(),
		StrategyID: strategy.StrategyID,
		Timeout:    arg.Timeout,
		ResReq:     arg.Spec.FindResourceRequestType(),
		ResReqSpec: arg.Spec,
		// HookCtl:    scanninghooks,
	}, nil
}

func ToScanningHookCtl(req *types.OpenAPIWebhookSetting) (*models.ScanningHookCtl, error) {
	if req == nil || req.Enabled == false {
		return &models.ScanningHookCtl{
			Enabled: false,
			Items:   nil,
		}, nil
	}

	ret := make([]*models.ScanningHook, 0)

	for _, hook := range req.HookList {
		repoInfo, err := mongodb.NewCodehostColl().GetSystemCodeHostByAlias(hook.CodeHostName)
		if err != nil {
			return nil, fmt.Errorf("failed to find codehost with name [%s], error is: %s", hook.CodeHostName, err)
		}
		newHook := &models.ScanningHook{
			CodehostID:   repoInfo.ID,
			Source:       repoInfo.Type,
			RepoOwner:    hook.RepoNamespace,
			RepoName:     hook.RepoName,
			Branch:       hook.Branch,
			Events:       hook.Events,
			MatchFolders: hook.MatchFolders,
		}
		ret = append(ret, newHook)
	}
	return &models.ScanningHookCtl{
		Enabled: true,
		Items:   ret,
	}, nil
}

func ToScanningRepository(repo *types.OpenAPIRepoInput) (*types.Repository, error) {
	repoInfo, err := mongodb.NewCodehostColl().GetSystemCodeHostByAlias(repo.CodeHostName)
	if err != nil {
		return nil, fmt.Errorf("failed to find codehost with name [%s], error is: %s", repo.CodeHostName, err)
	}
	return &types.Repository{
		Source:        repoInfo.Type,
		RepoOwner:     repo.RepoNamespace,
		RepoNamespace: repo.RepoNamespace,
		RepoName:      repo.RepoName,
		Branch:        repo.Branch,
		PR:            repo.PR,
		PRs:           repo.PRs,
		EnableCommit:  repo.EnableCommit,
		CommitID:      repo.CommitID,
		CodehostID:    repoInfo.ID,
		// this is not a required field in openAPI in scanning, we will leave it as origin for now
		RemoteName: "origin",
	}, nil
}

func ToBuildRepository(repo *types.OpenAPIRepoInput) (*types.Repository, error) {
	repoInfo, err := mongodb.NewCodehostColl().GetSystemCodeHostByAlias(repo.CodeHostName)
	if err != nil {
		return nil, fmt.Errorf("failed to find codehost with name [%s], error is: %s", repo.CodeHostName, err)
	}
	return &types.Repository{
		Source:        repoInfo.Type,
		RepoOwner:     repo.RepoNamespace,
		RepoNamespace: repo.RepoNamespace,
		RepoName:      repo.RepoName,
		Branch:        repo.Branch,
		PR:            repo.PR,
		PRs:           repo.PRs,
		EnableCommit:  repo.EnableCommit,
		CommitID:      repo.CommitID,
		CodehostID:    repoInfo.ID,
		RemoteName:    repo.RemoteName,
		SubModules:    repo.SubModules,
		CheckoutPath:  repo.CheckoutPath,
	}, nil
}

func ToBuildInstalls(installs []*types.OpenAPIToolItem) []*models.Item {
	ret := make([]*models.Item, 0)
	for _, install := range installs {
		ret = append(ret, &models.Item{
			Name:    install.Name,
			Version: install.Version,
		})
	}
	return ret
}

func ToKeyValList(parameters []*types.ParameterSetting) models.KeyValList {
	ret := make([]*models.KeyVal, 0)
	for _, parameter := range parameters {
		ret = append(ret, &models.KeyVal{
			Key:          parameter.Key,
			Value:        parameter.DefaultValue,
			Type:         models.ParameterSettingType(parameter.Type),
			ChoiceOption: parameter.ChoiceOption,
			ChoiceValue:  parameter.ChoiceValue,
			IsCredential: parameter.IsCredential,
			Description:  parameter.Description,
		})
	}
	return models.KeyValList(ret)
}

func ToKeyVals(keyValues []*types.KeyValue) []*util.KeyValue {
	ret := make([]*util.KeyValue, 0)
	for _, keyValue := range keyValues {
		ret = append(ret, &util.KeyValue{
			Key:   keyValue.Key,
			Value: keyValue.Value,
		})
	}
	return ret
}

func ToOutputs(outputs []string) []*models.Output {
	ret := make([]*models.Output, 0)
	for _, output := range outputs {
		ret = append(ret, &models.Output{
			Name: output,
		})
	}
	return ret
}
