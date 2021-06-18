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

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/setting"
)

func (c *Client) ScheduleCall(api string, args interface{}, log *zap.SugaredLogger) error {
	url := fmt.Sprintf("%s/%s", c.APIBase, api)
	log.Info("start run scheduled task..")
	body, err := json.Marshal(args)
	if err != nil {
		log.Errorf("marshal json args error: %v", err)
		return err
	}
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		log.Errorf("create post request error : %v", err)
		return err
	}
	request.Header.Set("Authorization", fmt.Sprintf("%s %s", setting.TIMERAPIKEY, c.Token))
	var resp *http.Response
	resp, err = c.Conn.Do(request)
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		if _, err := ioutil.ReadAll(resp.Body); err != nil {
			log.Errorf("run %s result %v", api, err)
		}
	}
	return err
}
