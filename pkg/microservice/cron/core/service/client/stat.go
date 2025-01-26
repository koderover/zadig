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
	"time"

	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/v2/pkg/config"
)

func (c *Client) InitStatData(log *zap.SugaredLogger) error {
	//build
	url := fmt.Sprintf("%s/api/stat/quality/initBuildStat", configbase.AslanServiceAddress())
	log.Info("start init buildStat..")
	_, err := c.sendPostRequest(url, nil, log)
	if err != nil {
		log.Errorf("trigger init buildStat error :%v", err)
	}
	//test
	url = fmt.Sprintf("%s/api/stat/quality/initTestStat", configbase.AslanServiceAddress())
	log.Info("start init testStat..")
	_, err = c.sendPostRequest(url, nil, log)
	if err != nil {
		log.Errorf("trigger init testStat error :%v", err)
	}

	// deploy now has 2 parts: weekly and monthly, do those separately when the time is right.

	// if it is a monday, do the weekly deploy stats
	if time.Now().Weekday() == time.Monday {
		url = fmt.Sprintf("%s/api/stat/v2/quality/deploy/weekly", configbase.AslanServiceAddress())
		log.Info("start creating weekly deploy stats..")
		_, err = c.sendPostRequest(url, nil, log)
		if err != nil {
			log.Errorf("creating weekly deploy stats error :%v", err)
		}
	}

	// if it is the first day of a month, do the monthly deploy stats
	if time.Now().Day() == 1 {
		url = fmt.Sprintf("%s/api/stat/v2/quality/deploy/monthly", configbase.AslanServiceAddress())
		log.Info("start creating monthly deploy stats..")
		_, err = c.sendPostRequest(url, nil, log)
		if err != nil {
			log.Errorf("creating monthly deploy stats error :%v", err)
		}
	}

	// if it is the first day of a month, do the monthly release stats
	if time.Now().Day() == 1 {
		url = fmt.Sprintf("%s/api/stat/v2/quality/release/weekly", configbase.AslanServiceAddress())
		log.Info("start creating monthly deploy stats..")
		_, err = c.sendPostRequest(url, nil, log)
		if err != nil {
			log.Errorf("creating monthly deploy stats error :%v", err)
		}
	}

	return nil
}
