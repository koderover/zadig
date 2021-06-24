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

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
)

func LogProductStats(user, event, prodName, requestID string, startTime int64, log *zap.SugaredLogger) {
	ctx := make(map[string]string)
	ctx["ProductName"] = prodName

	stats := &models.Stats{
		ReqID:     requestID,
		User:      user,
		Event:     event,
		StartTime: startTime,
		EndTime:   time.Now().Unix(),
		Context:   ctx,
	}

	if err := mongodb.NewStatsColl().Create(stats); err != nil {
		log.Errorf("[LogProductStats] error: %v", err)
	}
}
