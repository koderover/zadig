/*
Copyright 2023 The KodeRover Authors.

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

package imnotify

import (
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type IMNotifyType string

const (
	msgType    = "markdown"
	singleInfo = "single"
	multiInfo  = "multi"

	IMNotifyTypeDingDing IMNotifyType = "dingding"
	IMNotifyTypeWeChat   IMNotifyType = "wechat"
	IMNotifyTypeLark     IMNotifyType = "feishu"
)

type IMNotifyService struct {
	proxyColl *mongodb.ProxyColl
}

func NewIMNotifyClient() *IMNotifyService {
	return &IMNotifyService{
		proxyColl: mongodb.NewProxyColl(),
	}
}

func (w *IMNotifyService) SendMessageRequest(uri string, message interface{}) ([]byte, error) {
	c := httpclient.New()

	// 使用代理
	proxies, _ := w.proxyColl.List(&mongodb.ProxyArgs{})
	if len(proxies) != 0 && proxies[0].EnableApplicationProxy {
		c.SetProxy(proxies[0].GetProxyURL())
		log.Infof("send im notify message is using proxy:%s\n", proxies[0].GetProxyURL())
	}

	res, err := c.Post(uri, httpclient.SetBody(message))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}
