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

package models

import "github.com/koderover/zadig/pkg/util"

type StatDashboardConfig struct {
	ID        string     `bson:"_id,omitempty"`
	Type      string     `bson:"type"`
	ItemKey   string     `bson:"item_key"`
	Name      string     `bson:"name"`
	Source    string     `bson:"source"`
	APIConfig *APIConfig `bson:"api_config,omitempty"`
	Function  string     `bson:"function"`
	Weight    int64      `bson:"weight"`
}

type APIConfig struct {
	ExternalSystemId string           `json:"external_system_id"`
	ApiPath          string           `json:"api_path"`
	Queries          []*util.KeyValue `json:"queries"`
}

func (StatDashboardConfig) TableName() string {
	return "stat_dashboard_config"
}
