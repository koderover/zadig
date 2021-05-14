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
	"net/http"
	"net/url"
	"time"

	conf "github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func SetProxyConfig() {
	proxies, err := commonrepo.NewProxyColl().List(&commonrepo.ProxyArgs{})
	if err != nil || len(proxies) == 0 {
		return
	}

	if !proxies[0].EnableRepoProxy {
		conf.SetProxy("", "", "")
		return
	}

	url := proxies[0].GetProxyUrl()

	if proxies[0].Type == "http" {
		conf.SetProxy(url, url, "")
	} else if proxies[0].Type == "socks5" {
		conf.SetProxy(url, "", url)
	}
}

func ListProxies(log *xlog.Logger) ([]*commonmodels.Proxy, error) {
	resp, err := commonrepo.NewProxyColl().List(&commonrepo.ProxyArgs{})
	if err != nil {
		log.Errorf("Proxy.List error: %v", err)
		return resp, e.ErrListProxies.AddErr(err)
	}
	return resp, nil
}

func GetProxy(id string, log *xlog.Logger) (*commonmodels.Proxy, error) {
	resp, err := commonrepo.NewProxyColl().Find(id)
	if err != nil {
		log.Errorf("Proxy.Find %s error: %v", id, err)
		return resp, e.ErrGetProxy.AddErr(err)
	}
	return resp, nil
}

func CreateProxy(args *commonmodels.Proxy, log *xlog.Logger) error {
	err := commonrepo.NewProxyColl().Create(args)
	if err != nil {
		log.Errorf("Proxy.Create error: %v", err)
		return e.ErrCreateProxy.AddErr(err)
	}

	// 更新globalConfig的proxy配置
	SetProxyConfig()

	return nil
}

func UpdateProxy(id string, args *commonmodels.Proxy, log *xlog.Logger) error {
	err := commonrepo.NewProxyColl().Update(id, args)
	if err != nil {
		log.Errorf("Proxy.Update %s error: %v", id, err)
		return e.ErrUpdateProxy.AddErr(err)
	}

	// 更新globalConfig的proxy配置
	SetProxyConfig()

	return nil
}

func DeleteProxy(id string, log *xlog.Logger) error {
	err := commonrepo.NewProxyColl().Delete(id)
	if err != nil {
		log.Errorf("Proxy.Delete %s error: %v", id, err)
		return e.ErrDeleteProxy.AddErr(err)
	}
	return nil
}

func TestConnection(args *commonmodels.Proxy, log *xlog.Logger) error {
	if args == nil {
		log.Error("invalid args")
		return e.ErrTestConnection.AddDesc("invalid args")
	}

	request, err := http.NewRequest("GET", "https://www.baidu.com", nil)
	if err != nil {
		log.Errorf("http.NewRequest failed, err:%v", err)
		return e.ErrTestConnection.AddErr(err)
	}

	var ret *http.Response
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	p, err := url.Parse(args.GetProxyUrl())
	if err == nil {
		proxy := http.ProxyURL(p)
		trans := &http.Transport{
			Proxy: proxy,
		}
		client.Transport = trans
	}

	ret, err = client.Do(request)
	if err != nil {
		log.Errorf("client.Do failed, err:%v", err)
		return e.ErrTestConnection.AddErr(err)
	}
	defer func() { _ = ret.Body.Close() }()

	if ret.StatusCode != 200 {
		return e.ErrTestConnection
	}

	return nil
}
