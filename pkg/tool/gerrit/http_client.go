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

package gerrit

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type HTTPClient struct {
	*httpclient.Client

	host  string
	token string
}

func NewHTTPClient(host, token string) *HTTPClient {
	c := httpclient.New(
		httpclient.SetAuthScheme("Basic"),
		httpclient.SetAuthToken(token),
		httpclient.SetHostURL(host),
	)

	return &HTTPClient{
		Client: c,
		host:   host,
		token:  token,
	}
}

func (c *HTTPClient) UpsertWebhook(repoName, webhookName, webhookURL string, events []string) error {
	url := fmt.Sprintf("/%s/%s/%s/%s", "a/config/server/webhooks~projects", Escape(repoName), "remotes", webhookName)
	if _, err := c.Get(url); err == nil {
		log.Infof("webhook %s already exists", webhookName)
		return nil

	}
	c.SetHeader("Content-Type", "application/json")
	//create webhook
	gerritWebhook := &Webhook{
		URL:       fmt.Sprintf("%s?name=%s", webhookURL, webhookName),
		MaxTries:  setting.MaxTries,
		SslVerify: false,
	}
	for _, event := range events {
		gerritWebhook.Events = append(gerritWebhook.Events, string(event))
	}
	if _, err := c.Put(url, httpclient.SetBody(gerritWebhook)); err != nil {
		return fmt.Errorf("create gerrit webhook err:%v", err)
	}
	return nil
}

func (c *HTTPClient) DeleteWebhook(repoName, webhookName string) error {
	c.SetHeader("Content-Type", "text/plain;charset=utf-8")
	webhookURLPrefix := fmt.Sprintf("/%s/%s/%s", "a/config/server/webhooks~projects", Escape(repoName), "remotes")
	_, _ = c.Delete(fmt.Sprintf("%s/%s", webhookURLPrefix, RemoteName))
	if _, err := c.Delete(fmt.Sprintf("%s/%s", webhookURLPrefix, webhookName)); err != nil {
		return fmt.Errorf("delete gerrit webhook:%s err:%v", webhookName, err)
	}
	return nil
}
