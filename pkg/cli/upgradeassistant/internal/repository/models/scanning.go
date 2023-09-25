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

package models

import (
	"github.com/koderover/zadig/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Scanning struct {
	ID              primitive.ObjectID       `bson:"_id,omitempty"  json:"id,omitempty"`
	ImageID         string                   `bson:"image_id"       json:"image_id"`
	EnableScanner   bool                     `bson:"enable_scanner" json:"enable_scanner"`
	ScannerType     string                   `bson:"scanner_type"   json:"scanner_type"`
	PreScript       string                   `bson:"pre_script"       json:"pre_script"`
	Script          string                   `bson:"script"           json:"script"`
	AdvancedSetting *ScanningAdvancedSetting `bson:"advanced_setting" json:"advanced_setting"`
}

type ScanningAdvancedSetting struct {
	Cache *ScanningCacheSetting `bson:"cache"        json:"cache"`
}

type ScanningCacheSetting struct {
	CacheEnable  bool               `bson:"cache_enable"        json:"cache_enable"`
	CacheDirType types.CacheDirType `bson:"cache_dir_type"      json:"cache_dir_type"`
	CacheUserDir string             `bson:"cache_user_dir"      json:"cache_user_dir"`
}

func (Scanning) TableName() string {
	return "scanning"
}
