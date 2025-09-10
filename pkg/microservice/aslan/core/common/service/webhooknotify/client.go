/*
Copyright 2024 The KodeRover Authors.

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

package webhooknotify

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
)

type webhookNotifyclient struct {
	Token   string
	Address string
}

func NewClient(address, token string) *webhookNotifyclient {
	return &webhookNotifyclient{
		Token:   token,
		Address: address,
	}
}

func (c *webhookNotifyclient) SendWorkflowWebhook(webhookNotify *WorkflowNotify) error {
	notify := &WebHookNotify{
		ObjectKind: WebHookNotifyObjectKindWorkflow,
		Event:      WebHookNotifyEventWorkflow,
		Workflow:   webhookNotify,
	}
	return c.sendWebhook(notify)
}

func (c *webhookNotifyclient) SendReleasePlanWebhook(webhookNotify *ReleasePlanHookBody) error {
	notify := &WebHookNotify{
		ObjectKind:  WebHookNotifyObjectKindReleasePlan,
		Event:       WebHookNotifyEventReleasePlan,
		ReleasePlan: webhookNotify,
	}
	return c.sendWebhook(notify)
}

func (c *webhookNotifyclient) sendWebhook(notify *WebHookNotify) error {
	resp, err := httpclient.Post(
		c.Address,
		httpclient.SetBody(notify),
		httpclient.SetHeader(TokenHeader, c.Token),
		httpclient.SetHeader(InstanceHeader, config.SystemAddress()),
		httpclient.SetHeader(EventHeader, string(notify.Event)),
		httpclient.SetHeader(EventUUIDHeader, uuid.New().String()),
		httpclient.SetHeader(WebhookUUIDHeader, uuid.New().String()),
	)
	if err != nil {
		return fmt.Errorf("failed to execute post http request, url: %s, error: %v", c.Address, err)
	}

	if resp.IsError() {
		err := httpclient.NewErrorFromRestyResponse(resp)
		return err
	}

	if resp.IsSuccess() {
		return nil
	}
	return nil
}
