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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/types"
)

type Testing struct {
	ID      primitive.ObjectID `bson:"_id,omitempty"            json:"id,omitempty"`
	PreTest *PreTest           `bson:"pre_test"                 json:"pre_test"`

	// TODO: Deprecated.
	Caches []string `bson:"caches"                   json:"caches"`

	// New since V1.10.0.
	CacheEnable  bool               `bson:"cache_enable"        json:"cache_enable"`
	CacheDirType types.CacheDirType `bson:"cache_dir_type"      json:"cache_dir_type"`
	CacheUserDir string             `bson:"cache_user_dir"      json:"cache_user_dir"`
}

// PreTest prepares an environment for a job
type PreTest struct {
	// TODO: Deprecated.
	CleanWorkspace bool `bson:"clean_workspace"            json:"clean_workspace"`
}

func (Testing) TableName() string {
	return "module_testing"
}
