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

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/koderover/zadig/pkg/microservice/cron/core/service"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (c *Client) ListPmHosts(log *zap.SugaredLogger) ([]*service.PrivateKeyHosts, error) {
	var (
		err  error
		resp = make([]*service.PrivateKeyHosts, 0)
	)

	url := fmt.Sprintf("%s/system/privateKey/internal", c.APIBase)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("ListPmHosts new http request error: %v", err)
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
	return resp, errors.WithMessage(err, "failed to list PmHosts")
}

func (c *Client) UpdatePmHost(args *service.PrivateKeyHosts, log *zap.SugaredLogger) error {
	args.UpdateStatus = true
	body, err := json.Marshal(args)
	if err != nil {
		log.Errorf("marshal json args error: %v", err)
		return err
	}

	url := fmt.Sprintf("%s/system/privateKey/%s", c.APIBase, args.ID.Hex())
	request, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		log.Errorf("UpdatePmHost new http request error: %v", err)
		return err
	}

	var ret *http.Response
	if ret, err = c.Conn.Do(request); err == nil {
		defer func() { _ = ret.Body.Close() }()
		_, err := ioutil.ReadAll(ret.Body)
		if err != nil {
			return errors.WithMessage(err, "failed to get PmHost")
		}
	}

	return nil
}
