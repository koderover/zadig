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

	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/pkg/types"
)

func ToScanningAdvancedSetting(arg *types.OpenAPIAdvancedSetting) (*types.ScanningAdvancedSetting, error) {
	cluster, err := commonrepo.NewK8SClusterColl().FindByName(arg.ClusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to find cluster of name: %s, the error is: %s", arg.ClusterName, err)
	}

	scanninghooks, err := ToScanningHookCtl(arg.Webhooks)
	if err != nil {
		return nil, err
	}

	return &types.ScanningAdvancedSetting{
		ClusterID:  cluster.ID.Hex(),
		Timeout:    arg.Timeout,
		ResReq:     arg.Spec.FindResourceRequestType(),
		ResReqSpec: arg.Spec,
		HookCtl:    scanninghooks,
	}, nil
}

func ToScanningHookCtl(req *types.OpenAPIWebhookSetting) (*types.ScanningHookCtl, error) {
	if req == nil || req.Enabled == false {
		return &types.ScanningHookCtl{
			Enabled: false,
			Items:   nil,
		}, nil
	}

	ret := make([]*types.ScanningHook, 0)

	for _, hook := range req.HookList {
		repoInfo, err := mongodb.NewCodehostColl().GetCodeHostByAlias(hook.CodeHostName)
		if err != nil {
			return nil, fmt.Errorf("failed to find codehost with name [%s], error is: %s", hook.CodeHostName, err)
		}
		newHook := &types.ScanningHook{
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
	return &types.ScanningHookCtl{
		Enabled: true,
		Items:   ret,
	}, nil
}

func ToScanningRepository(repo *types.OpenAPIRepoInput) (*types.Repository, error) {
	repoInfo, err := mongodb.NewCodehostColl().GetCodeHostByAlias(repo.CodeHostName)
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
		CodehostID:    repoInfo.ID,
		// this is not a required field in openAPI in scanning, we will leave it as origin for now
		RemoteName: "origin",
	}, nil
}

func ToBuildRepository(repo *types.OpenAPIRepoInput) (*types.Repository, error) {
	repoInfo, err := mongodb.NewCodehostColl().GetCodeHostByAlias(repo.CodeHostName)
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
		CodehostID:    repoInfo.ID,
		RemoteName:    repo.RemoteName,
		SubModules:    repo.SubModules,
		CheckoutPath:  repo.CheckoutPath,
	}, nil
}
