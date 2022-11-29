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
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/cron/core/service"
)

type EvnListOption struct {
	BasicFacility string
	DeployType    []string
}

func (c *Client) ListEnvs(log *zap.SugaredLogger, option *EvnListOption) ([]*service.ProductRevision, error) {
	var (
		err  error
		resp = make([]*service.ProductRevision, 0)
	)

	url := fmt.Sprintf("%s/environment/revision/products?basicFacility=%s&deployType=%s", c.APIBase, option.BasicFacility, strings.Join(option.DeployType, ","))
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("ListEnvs new http request error: %v", err)
		return nil, err
	}

	var ret *http.Response
	if ret, err = c.Conn.Do(request); err == nil {
		defer func() { _ = ret.Body.Close() }()
		var body []byte
		body, err = io.ReadAll(ret.Body)
		if err == nil {
			if err = json.Unmarshal(body, &resp); err == nil {
				return resp, nil
			}
		}
	}
	return resp, errors.WithMessage(err, "failed to list envs")
}

func (c *Client) ListEnvResources(productName, envName string, log *zap.SugaredLogger) ([]*service.EnvResource, error) {
	var (
		err  error
		resp = make([]*service.EnvResource, 0)
	)
	url := fmt.Sprintf("%s/environment/envcfgs?autoSync=true&projectName=%s&envName=%s", c.APIBase, productName, envName)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("ListEnvResources new http request error: %s", err)
		return nil, err
	}

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

	return resp, err
}

func (c *Client) GetEnvService(productName, envName string, log *zap.SugaredLogger) (*service.ProductResp, error) {
	var (
		err    error
		envObj = new(service.ProductResp)
	)
	url := fmt.Sprintf("%s/environment/environments/%s?projectName=%s", c.APIBase, envName, productName)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("GetService new http request error: %v", err)
		return nil, err
	}

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

func (c *Client) GetRenderset(name string, revision int64, log *zap.SugaredLogger) (*service.ProductRenderset, error) {
	var (
		err          error
		rendersetObj = new(service.ProductRenderset)
	)
	url := fmt.Sprintf("%s/project/renders/render/%s/revision/%d", c.APIBase, name, revision)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("GetService new http request error: %v", err)
		return nil, err
	}

	var ret *http.Response
	if ret, err = c.Conn.Do(request); err == nil {
		defer func() { _ = ret.Body.Close() }()
		var body []byte
		body, err = ioutil.ReadAll(ret.Body)
		if err == nil {
			if err = json.Unmarshal(body, &rendersetObj); err == nil {
				return rendersetObj, nil
			}
		}
	}

	return rendersetObj, errors.WithMessage(err, "failed to get renderset")
}

func (c *Client) GetService(serviceName, productName, serviceType string, revision int64, log *zap.SugaredLogger) (*service.Service, error) {
	var (
		err     error
		service = new(service.Service)
	)
	url := fmt.Sprintf("%s/service/services/%s/%s?productName=%s&revision=%d", c.APIBase, serviceName, serviceType, productName, revision)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("GetService new http request error: %v", err)
		return nil, err
	}

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

func (c *Client) UpdateService(args *service.ServiceTmplObject, log *zap.SugaredLogger) error {
	body, err := json.Marshal(args)
	if err != nil {
		log.Errorf("marshal json args error: %v", err)
		return err
	}

	url := fmt.Sprintf("%s/service/services", c.APIBase)
	request, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		log.Errorf("UpdateService new http request error: %v", err)
		return err
	}

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

func (c *Client) SyncEnvVariables(productName, envName string, log *zap.SugaredLogger) error {
	url := fmt.Sprintf("%s/environment/environments/%s/syncVariables?projectName=%s", c.APIBase, envName, productName)
	request, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		log.Errorf("SyncEnvVariables new http request error: %v", err)
		return err
	}

	var ret *http.Response
	if ret, err = c.Conn.Do(request); err == nil {
		defer func() { _ = ret.Body.Close() }()
		_, err := ioutil.ReadAll(ret.Body)
		if err != nil {
			return errors.WithMessage(err, "failed to sync product variables ")
		}
	}
	return nil
}

func (c *Client) SyncEnvResource(productName, envName, resType, resName string, log *zap.SugaredLogger) error {
	url := fmt.Sprintf("%s/environment/envcfgs/%s/%s/%s/sync?projectName=%s", c.APIBase, envName, resType, resName, productName)
	request, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		log.Errorf("SyncEnvResource new http request error: %v", err)
		return err
	}

	var ret *http.Response
	if ret, err = c.Conn.Do(request); err == nil {
		defer func() { _ = ret.Body.Close() }()
	}
	return nil
}

func (c *Client) GetHostInfo(hostID string, log *zap.SugaredLogger) (*service.PrivateKey, error) {
	var (
		err        error
		privateKey = new(service.PrivateKey)
	)
	url := fmt.Sprintf("%s/system/privateKey/%s", c.APIBase, hostID)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("GetPrivateKey new http request error: %v", err)
		return nil, err
	}

	var ret *http.Response
	if ret, err = c.Conn.Do(request); err == nil {
		defer func() { _ = ret.Body.Close() }()
		var body []byte
		body, err = ioutil.ReadAll(ret.Body)
		if err == nil {
			if err = json.Unmarshal(body, &privateKey); err == nil {
				return privateKey, nil
			}
		}
	}

	return privateKey, errors.WithMessage(err, "failed to get privateKey")
}
