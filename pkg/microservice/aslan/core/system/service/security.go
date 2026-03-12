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
	"encoding/json"
	"time"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	aslanclient "github.com/koderover/zadig/v2/pkg/shared/client/aslan"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"go.uber.org/zap"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

const securitySettingsCacheTTL = 30 * time.Second

func CreateOrUpdateSecuritySettings(args *SecurityAndPrivacySettings, logger *zap.SugaredLogger) error {
	err := commonrepo.NewSystemSettingColl().UpdateSecuritySetting(args.TokenExpirationTime, args.MFAEnabled)
	if err != nil {
		logger.Errorf("failed to update security settings, error: %s", err)
		return err
	}

	err = commonrepo.NewSystemSettingColl().UpdatePrivacySetting(args.ImprovementPlan)
	if err != nil {
		logger.Errorf("failed to update privacy settings, error: %s", err)
	}

	if cacheErr := syncSystemSecuritySettingsCache(logger); cacheErr != nil {
		logger.Warnf("failed to sync security settings cache: %v", cacheErr)
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
	var mfaEnabled bool
	if systemSetting.Security != nil {
		tokenExpirationTime = systemSetting.Security.TokenExpirationTime
		mfaEnabled = systemSetting.Security.MFAEnabled
	}

	var improvementPlan bool = true
	if systemSetting.Privacy != nil {
		improvementPlan = systemSetting.Privacy.ImprovementPlan
	}
	return &SecurityAndPrivacySettings{
		TokenExpirationTime: tokenExpirationTime,
		MFAEnabled:          mfaEnabled,
		ImprovementPlan:     improvementPlan,
	}, nil
}

func syncSystemSecuritySettingsCache(logger *zap.SugaredLogger) error {
	settings, err := GetSecuritySettings(logger)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(&aslanclient.SystemSetting{
		TokenExpirationTime: settings.TokenExpirationTime,
		MFAEnabled:          settings.MFAEnabled,
		ImprovementPlan:     settings.ImprovementPlan,
	})
	if err != nil {
		return err
	}

	return cache.NewRedisCache(configbase.RedisCommonCacheTokenDB()).Write(
		setting.SystemSecuritySettingsCacheKey,
		string(payload),
		securitySettingsCacheTTL,
	)
}
