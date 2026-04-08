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

package common

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/aslan"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"go.uber.org/zap"
)

const securitySettingsCacheTTL = 30 * time.Second

func GetSystemSecuritySettings(logger *zap.SugaredLogger) (*aslan.SystemSetting, error) {
	cachedSettings, err := getSystemSecuritySettingsFromCache()
	if err == nil && cachedSettings != nil {
		return cachedSettings, nil
	}
	if err != nil && !errors.Is(err, redis.Nil) && logger != nil {
		logger.Warnf("failed to read security settings cache from redis, will refresh from aslan: %v", err)
	}

	settings, err := aslan.New(configbase.AslanServiceAddress()).GetSystemSecurityAndPrivacySettings()
	if err != nil {
		return nil, err
	}

	if err = saveSystemSecuritySettingsToCache(settings); err != nil && logger != nil {
		logger.Warnf("failed to save security settings cache to redis: %v", err)
	}

	resp := *settings
	return &resp, nil
}

func getSystemSecuritySettingsFromCache() (*aslan.SystemSetting, error) {
	data, err := cache.NewRedisCache(configbase.RedisCommonCacheTokenDB()).GetString(setting.SystemSecuritySettingsCacheKey)
	if err != nil {
		return nil, err
	}
	resp := &aslan.SystemSetting{}
	if err := json.Unmarshal([]byte(data), resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func saveSystemSecuritySettingsToCache(settings *aslan.SystemSetting) error {
	if settings == nil {
		return errors.New("security settings are nil")
	}

	payload, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return cache.NewRedisCache(configbase.RedisCommonCacheTokenDB()).Write(
		setting.SystemSecuritySettingsCacheKey,
		string(payload),
		securitySettingsCacheTTL,
	)
}
