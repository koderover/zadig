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
	"fmt"

	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/pkg/config"
)

func (c *Client) InitStatData(log *zap.SugaredLogger) error {
	//build
	url := fmt.Sprintf("%s/api/quality/stat/initBuildStat", configbase.AslanxServiceAddress())
	log.Info("start init buildStat..")
	err := c.sendRequest(url)
	if err != nil {
		log.Errorf("trigger init buildStat error :%v", err)
	}
	//test
	url = fmt.Sprintf("%s/api/quality/stat/initTestStat", configbase.AslanxServiceAddress())
	log.Info("start init testStat..")
	err = c.sendRequest(url)
	if err != nil {
		log.Errorf("trigger init testStat error :%v", err)
	}
	//deploy
	url = fmt.Sprintf("%s/api/quality/stat/initDeployStat", configbase.AslanxServiceAddress())
	log.Info("start init deployStat..")
	err = c.sendRequest(url)
	if err != nil {
		log.Errorf("trigger init deployStat error :%v", err)
	}

	return nil
}

func (c *Client) InitOperationStatData(log *zap.SugaredLogger) error {
	//operation
	url := fmt.Sprintf("%s/api/operation/stat/initOperationStat", configbase.AslanxServiceAddress())
	log.Info("start init operationStat..")
	err := c.sendRequest(url)
	if err != nil {
		log.Errorf("trigger init operationStat error :%v", err)
	}

	if webHookUser, err := c.GetWebHookUser(log); err == nil {
		if err = c.CreateWebHookUser(webHookUser, log); err != nil {
			log.Errorf("CreateWebHookUser err:%v", err)
		}
	} else {
		log.Errorf("GetWebHookUser err:%v", err)
	}

	return err
}
