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

package webhook

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	gitservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/gitee"
)

func ProcessGiteeHook(payload []byte, req *http.Request, requestID string, log *zap.SugaredLogger) error {
	token := req.Header.Get("X-Gitee-Token")
	secret := gitservice.GetHookSecret()

	if secret != "" && token != secret {
		return errors.New("token is illegal")
	}

	eventType := gitee.HookEventType(req)
	event, err := gitee.ParseHook(eventType, payload)
	if err != nil {
		return err
	}

	baseURI := config.SystemAddress()
	var errorList = &multierror.Error{}

	switch event := event.(type) {
	case *gitee.PushEvent:
		//add webhook user
		if len(event.Commits) > 0 {
			webhookUser := &commonmodels.WebHookUser{
				Domain:    req.Header.Get("X-Forwarded-Host"),
				UserName:  event.Commits[0].Author.Name,
				Email:     event.Commits[0].Author.Email,
				Source:    setting.SourceFromGitee,
				CreatedAt: time.Now().Unix(),
			}
			commonrepo.NewWebHookUserColl().Upsert(webhookUser)
		}
		if err = TriggerWorkflowByGiteeEvent(event, baseURI, requestID, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}
	case *gitee.PullRequestEvent:
		if event.Action != "open" && event.Action != "update" {
			return fmt.Errorf("action %s is skipped", event.Action)
		}
		if err = TriggerWorkflowByGiteeEvent(event, baseURI, requestID, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}
	case *gitee.TagPushEvent:
		if err = TriggerWorkflowByGiteeEvent(event, baseURI, requestID, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}
	}

	return errorList.ErrorOrNil()
}
