/*
Copyright 2024 The KodeRover Authors.

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

package util

import (
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func GeneFlatUsers(users []*models.User) ([]*models.User, map[string]*types.UserInfo) {
	flatUsers := make([]*models.User, 0)
	userSet := sets.NewString()
	userMap := make(map[string]*types.UserInfo)

	if users == nil {
		return flatUsers, userMap
	}

	for _, u := range users {
		if u.Type == setting.UserTypeUser || u.Type == "" {
			userSet.Insert(u.UserID)
			flatUsers = append(flatUsers, u)
		}
	}
	for _, u := range users {
		if u.Type == setting.UserTypeGroup {
			groupInfo, err := user.New().GetGroupDetailedInfo(u.GroupID)
			if err != nil {
				log.Warnf("CreateNativeApproval GetGroupDetailedInfo error, error msg:%s", err)
				continue
			}
			for _, uid := range groupInfo.UIDs {
				if userSet.Has(uid) {
					continue
				}
				userSet.Insert(uid)
				userDetailedInfo, err := user.New().GetUserByID(uid)
				if err != nil {
					log.Errorf("failed to find user %s, error: %s", uid, err)
					continue
				}
				userMap[uid] = userDetailedInfo
				flatUsers = append(flatUsers, &models.User{
					Type:     setting.UserTypeUser,
					UserID:   uid,
					UserName: userDetailedInfo.Name,
				})
			}
		}
	}

	return flatUsers, userMap
}
