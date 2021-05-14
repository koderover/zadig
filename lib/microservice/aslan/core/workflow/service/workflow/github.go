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

package workflow

import (
	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/task"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	git "github.com/koderover/zadig/lib/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/github/app"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func createGitCheck(pt *task.Task, log *xlog.Logger) error {
	if pt.TaskCreator != setting.WebhookTaskCreator {
		return nil
	}

	var hook *commonmodels.HookPayload
	if pt.Type == config.SingleType {
		hook = pt.TaskArgs.HookPayload
	} else if pt.Type == config.WorkflowType {
		hook = pt.WorkflowArgs.HookPayload
	}

	if hook == nil {
		return nil
	}

	gitCli, err := initGithubClient(hook.Owner)
	if err != nil {
		log.Errorf("initGithubClient failed, err:%v", err)
		return e.ErrGithubUpdateStatus.AddErr(err)
	}

	if hook.IsPr {
		opt := &git.GitCheck{
			Owner:  hook.Owner,
			Repo:   hook.Repo,
			Branch: hook.Ref,
			Ref:    hook.Ref,
			IsPr:   hook.IsPr,

			AslanURL:    config.AslanURL(),
			PipeName:    pt.PipelineName,
			ProductName: pt.ProductName,
			PipeType:    pt.Type,
			TaskID:      pt.TaskID,
		}

		log.Infof("GitCheck opt ---> %+v", opt)
		checkID, err := gitCli.Checks.StartGitCheck(opt)
		if err != nil {
			return err
		}

		if pt.Type == config.SingleType {
			log.Infof("pipeline github check created, id: %d", checkID)
			pt.TaskArgs.HookPayload.CheckRunID = checkID
		} else if pt.Type == config.WorkflowType {
			log.Infof("workflow github check created, id: %d", checkID)
			pt.WorkflowArgs.HookPayload.CheckRunID = checkID
		}
	}

	return nil
}

func initGithubClient(owner string) (*git.Client, error) {
	githubApps, err := commonrepo.NewGithubAppColl().Find()
	if len(githubApps) == 0 {
		return nil, err
	}
	appKey := githubApps[0].AppKey
	appID := githubApps[0].AppID

	client, err := getGithubAppCli(appKey, appID)
	if client == nil {
		return nil, err
	}
	installID, err := client.FindInstallationID(owner)
	if err != nil {
		return nil, err
	}

	gitCfg := &git.Config{
		AppKey:         appKey,
		AppID:          appID,
		InstallationID: installID,
		ProxyAddr:      config.ProxyHTTPSAddr(),
	}

	appCli, err := git.NewDynamicClient(gitCfg)
	if err != nil {
		return nil, err
	}

	return appCli, nil
}

func getGithubAppCli(appKey string, appID int) (*app.Client, error) {
	return app.NewAppClient(&app.Config{
		AppKey: appKey,
		AppID:  appID,
	})
}
