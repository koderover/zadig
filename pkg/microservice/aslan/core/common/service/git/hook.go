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

package git

import (
	"fmt"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func GetSystemServerURL() (string, error) {
	setting, err := commonrepo.NewSystemSettingColl().Get()
	if err != nil {
		err = fmt.Errorf("failed to get system setting: %w", err)
		return "", err
	}

	serverURL := configbase.SystemAddress()
	if setting.ServerURL != "" {
		serverURL = setting.ServerURL
	}

	return serverURL, nil
}

func WebHookURL() string {
	serverURL, err := GetSystemServerURL()
	if err != nil {
		log.Errorf("failed to get system server url: %s", err)
		return fmt.Sprintf("%s/api/aslan/webhook", configbase.SystemAddress())
	}
	return fmt.Sprintf("%s/api/aslan/webhook", serverURL)
}
