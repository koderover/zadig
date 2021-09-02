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

package poetry

import (
	"fmt"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type userView struct {
	User *UserInfo `json:"info"`
}

func GetUserDetailByToken(poetryHost, token string) (*UserInfo, error) {
	url := "/directory/user/detail"

	cl := httpclient.New(httpclient.SetHostURL(poetryHost))

	//根据token获取用户
	userViewInfo := &userView{}
	_, err := cl.Get(url, httpclient.SetHeader(setting.AuthorizationHeader, fmt.Sprintf("%s %s", setting.UserAPIKey, token)), httpclient.SetResult(userViewInfo))
	if err != nil {
		return nil, err
	}

	return userViewInfo.User, nil
}
