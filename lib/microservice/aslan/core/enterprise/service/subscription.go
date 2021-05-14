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
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/notify"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func UpsertSubscription(user string, subscription *commonmodels.Subscription, log *xlog.Logger) error {
	if err := notify.NewNotifyClient().UpsertSubscription(user, subscription); err != nil {
		log.Errorf("NotifyCli.Subscribe error: %v", err)
		return e.ErrSubscribeNotify
	}
	return nil
}

func UpdateSubscribe(user string, notifyType int, subscription *commonmodels.Subscription, log *xlog.Logger) error {
	if err := notify.NewNotifyClient().UpdateSubscribe(user, notifyType, subscription); err != nil {
		log.Errorf("NotifyCli.UpdateSubscribe error: %v", err)
		return e.ErrUpdateSubscribe
	}
	return nil
}

func Unsubscribe(user string, notifyType int, log *xlog.Logger) error {
	if err := notify.NewNotifyClient().Unsubscribe(user, notifyType); err != nil {
		log.Errorf("NotifyCli.Unsubscribe error: %v", err)
		return e.ErrUnsubscribeNotify
	}
	return nil
}

func ListSubscriptions(user string, log *xlog.Logger) ([]*commonmodels.Subscription, error) {
	resp, err := notify.NewNotifyClient().ListSubscriptions(user)
	if err != nil {
		log.Errorf("NotifyCli.ListSubscriptions error: %v", err)
		return resp, e.ErrListSubscriptions
	}
	return resp, nil
}
