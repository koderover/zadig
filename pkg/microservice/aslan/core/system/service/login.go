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

package service

import (
	"go.uber.org/zap"

	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
)

func GetDefaultLogin() (*GetDefaultLoginResponse, error) {
	configuration, err := commonrepo.NewSystemSettingColl().Get()
	if err != nil {
		return nil, err
	}
	defaultLogin := setting.DefaultLoginLocal
	if len(configuration.DefaultLogin) > 0 {
		defaultLogin = configuration.DefaultLogin
	}
	return &GetDefaultLoginResponse{
		DefaultLogin: defaultLogin,
	}, nil
}

type GetDefaultLoginResponse struct {
	DefaultLogin string `json:"defaultLogin"`
}

type UpdateDefaultLoginParams struct {
	DefaultLogin string `json:"defaultLogin"`
}

func UpdateDefaultLogin(defaultLogin string, log *zap.SugaredLogger) error {
	return commonrepo.NewSystemSettingColl().UpdateDefaultLoginSetting(defaultLogin)
}
