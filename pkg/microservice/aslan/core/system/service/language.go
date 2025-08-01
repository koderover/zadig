/*
Copyright 2025 The KodeRover Authors.

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

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

func GetSystemLanguage(log *zap.SugaredLogger) (string, error) {
	setting, err := commonrepo.NewSystemSettingColl().Get()
	if err != nil {
		log.Errorf("get theme infos error: %v", err)
		return "", err
	}

	language := "zh-CN"
	if setting.Language != "" {
		language = setting.Language
	}

	return language, nil
}

func SetSystemLanguage(language string, log *zap.SugaredLogger) error {
	return commonrepo.NewSystemSettingColl().UpdateLanguage(language)
}
