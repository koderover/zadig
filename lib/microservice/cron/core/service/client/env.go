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

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/lib/microservice/cron/core/service"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func (c *Client) ListEnvs(log *xlog.Logger) ([]*service.ProductRevision, error) {

	var (
		err  error
		resp = make([]*service.ProductRevision, 0)
	)

	url := fmt.Sprintf("%s/revision/products", c.ApiBase)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("ListEnvs new http request error: %v", err)
		return nil, err
	}

	request.Header.Set("Authorization", fmt.Sprintf("%s %s", setting.ROOTAPIKEY, c.Token))
	var ret *http.Response
	if ret, err = c.Conn.Do(request); err == nil {
		defer func() { _ = ret.Body.Close() }()
		var body []byte
		body, err = ioutil.ReadAll(ret.Body)
		if err == nil {
			if err = json.Unmarshal(body, &resp); err == nil {
				return resp, nil
			}
		}
	}

	return resp, errors.WithMessage(err, "failed to list envs")
}

func (c *Client) GetEnvService(productName, envName string, log *xlog.Logger) (*service.ProductResp, error) {
	var (
		err    error
		envObj = new(service.ProductResp)
	)
	url := fmt.Sprintf("%s/products/%s?envName=%s", c.ApiBase, productName, envName)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("GetService new http request error: %v", err)
		return nil, err
	}

	request.Header.Set("Authorization", fmt.Sprintf("%s %s", setting.ROOTAPIKEY, c.Token))
	var ret *http.Response
	if ret, err = c.Conn.Do(request); err == nil {
		defer func() { _ = ret.Body.Close() }()
		var body []byte
		body, err = ioutil.ReadAll(ret.Body)
		if err == nil {
			if err = json.Unmarshal(body, &envObj); err == nil {
				return envObj, nil
			}
		}
	}

	return envObj, errors.WithMessage(err, "failed to get service")
}

func (c *Client) GetService(serviceName, productName, serviceType string, revision int64, log *xlog.Logger) (*service.Service, error) {
	var (
		err     error
		service = new(service.Service)
	)
	url := fmt.Sprintf("%s/service/services/%s/%s?productName=%s&revision=%d", c.ApiBase, serviceName, serviceType, productName, revision)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("GetService new http request error: %v", err)
		return nil, err
	}

	request.Header.Set("Authorization", fmt.Sprintf("%s %s", setting.ROOTAPIKEY, c.Token))
	var ret *http.Response
	if ret, err = c.Conn.Do(request); err == nil {
		defer func() { _ = ret.Body.Close() }()
		var body []byte
		body, err = ioutil.ReadAll(ret.Body)
		if err == nil {
			if err = json.Unmarshal(body, &service); err == nil {
				return service, nil
			}
		}
	}

	return service, errors.WithMessage(err, "failed to get service")
}

func (c *Client) UpdateService(args *service.ServiceTmplObject, log *xlog.Logger) error {
	body, err := json.Marshal(args)
	if err != nil {
		log.Errorf("marshal json args error: %v", err)
		return err
	}

	url := fmt.Sprintf("%s/service/services", c.ApiBase)
	request, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		log.Errorf("UpdateService new http request error: %v", err)
		return err
	}

	request.Header.Set("Authorization", fmt.Sprintf("%s %s", setting.ROOTAPIKEY, c.Token))
	var ret *http.Response
	if ret, err = c.Conn.Do(request); err == nil {
		defer func() { _ = ret.Body.Close() }()
		_, err := ioutil.ReadAll(ret.Body)
		if err != nil {
			return errors.WithMessage(err, "failed to get service")
		}
	}

	return nil
}
