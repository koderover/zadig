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
	"time"

	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func LogProductStats(user, event, prodName string, startTime int64, log *xlog.Logger) {
	ctx := make(map[string]string)
	ctx["ProductName"] = prodName

	stats := &models.Stats{
		ReqID:     log.ReqID(),
		User:      user,
		Event:     event,
		StartTime: startTime,
		EndTime:   time.Now().Unix(),
		Context:   ctx,
	}

	if err := repo.NewStatsColl().Create(stats); err != nil {
		log.Errorf("[LogProductStats] error: %v", err)
	}
}
