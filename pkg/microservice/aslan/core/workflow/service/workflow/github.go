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
	"fmt"

	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func createGitCheck(pt *task.Task, log *zap.SugaredLogger) error {
	if pt.TaskCreator != setting.WebhookTaskCreator {
		return nil
	}

	var hook *commonmodels.HookPayload
	if pt.Type == config.SingleType {
		hook = pt.TaskArgs.HookPayload
	} else if pt.Type == config.WorkflowType {
		hook = pt.WorkflowArgs.HookPayload
	}

	if hook == nil || !hook.IsPr {
		return nil
	}

	ghApp, err := github.GetGithubAppClientByOwner(hook.Owner)
	if err != nil {
		log.Errorf("getGithubAppClient failed, err:%v", err)
		return e.ErrGithubUpdateStatus.AddErr(err)
	}
	if ghApp != nil {
		log.Infof("GitHub App found, start to create check-run")
		opt := &github.GitCheck{
			Owner:  hook.Owner,
			Repo:   hook.Repo,
			Branch: hook.Ref,
			Ref:    hook.Ref,
			IsPr:   hook.IsPr,

			AslanURL:    configbase.SystemAddress(),
			PipeName:    pt.PipelineName,
			DisplayName: getDisplayName(pt),
			ProductName: pt.ProductName,
			PipeType:    pt.Type,
			TaskID:      pt.TaskID,
		}
		checkID, err := ghApp.StartGitCheck(opt)
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

		return nil
	}

	log.Infof("Init GitHub status")
	ch, err := systemconfig.New().GetCodeHost(pt.TriggerBy.CodehostID)
	if err != nil {
		log.Errorf("Failed to get codeHost, err:%v", err)
		return e.ErrGithubUpdateStatus.AddErr(err)
	}
	gc := github.NewClient(ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)

	return gc.UpdateCheckStatus(&github.StatusOptions{
		Owner:       hook.Owner,
		Repo:        hook.Repo,
		Ref:         hook.Ref,
		State:       github.StatePending,
		Description: fmt.Sprintf("Workflow [%s] is queued.", pt.PipelineDisplayName),
		AslanURL:    configbase.SystemAddress(),
		PipeName:    pt.PipelineName,
		DisplayName: getDisplayName(pt),
		ProductName: pt.ProductName,
		PipeType:    pt.Type,
		TaskID:      pt.TaskID,
	})
}

func getDisplayName(pt *task.Task) string {
	if pt.PipelineDisplayName != "" {
		return pt.PipelineDisplayName
	}
	return pt.PipelineName
}
