/*
Copyright 2023 The KodeRover Authors.

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

func CreateOrUpdateSecuritySettings(args *SecurityAndPrivacySettings, logger *zap.SugaredLogger) error {
	err := commonrepo.NewSystemSettingColl().UpdateSecuritySetting(args.TokenExpirationTime)
	if err != nil {
		logger.Errorf("failed to update security settings, error: %s", err)
		return err
	}

	err = commonrepo.NewSystemSettingColl().UpdatePrivacySetting(args.ImprovementPlan)
	if err != nil {
		logger.Errorf("failed to update privacy settings, error: %s", err)
	}

	return err
}

func GetSecuritySettings(logger *zap.SugaredLogger) (*SecurityAndPrivacySettings, error) {
	systemSetting, err := commonrepo.NewSystemSettingColl().Get()
	if err != nil {
		logger.Errorf("failed to get system settings, error: %s", err)
		return nil, err
	}
	var tokenExpirationTime int64 = 24
	if systemSetting.Security != nil {
		tokenExpirationTime = systemSetting.Security.TokenExpirationTime
	}

	var improvementPlan bool = true
	if systemSetting.Privacy != nil {
		improvementPlan = systemSetting.Privacy.ImprovementPlan
	}
	return &SecurityAndPrivacySettings{
		TokenExpirationTime: tokenExpirationTime,
		ImprovementPlan:     improvementPlan,
	}, nil
}
