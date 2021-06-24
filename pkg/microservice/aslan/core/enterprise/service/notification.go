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
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/notify"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func DeleteNotifies(user string, notifyIDs []string, log *zap.SugaredLogger) error {
	if err := notify.NewNotifyClient().DeleteNotifies(user, notifyIDs); err != nil {
		log.Errorf("NotifyCli.DeleteNotifies error: %v", err)
		return e.ErrDeleteNotifies
	}
	return nil
}

func PullNotify(user string, log *zap.SugaredLogger) ([]*commonmodels.Notify, error) {
	resp, err := notify.NewNotifyClient().PullNotify(user)
	if err != nil {
		log.Errorf("NotifyCli.PullNotify error: %v", err)
		return resp, e.ErrPullNotify
	}
	return resp, nil
}

func ReadNotify(user string, notifyIDs []string, log *zap.SugaredLogger) error {
	if err := notify.NewNotifyClient().Read(user, notifyIDs); err != nil {
		log.Errorf("NotifyCli.Read error: %v", err)
		return e.ErrReadNotify
	}
	return nil
}
