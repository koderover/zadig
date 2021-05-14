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

package models

type Install struct {
	ObjectIdHex  string   `bson:"-"                      json:"-"`
	Name         string   `bson:"name"                   json:"name"`
	Version      string   `bson:"version"                json:"version"`
	Scripts      string   `bson:"scripts"                json:"scripts"`
	UpdateTime   int64    `bson:"update_time"            json:"update_time"`
	UpdateBy     string   `bson:"update_by"              json:"update_by"`
	Envs         []string `bson:"env"                    json:"env"`
	BinPath      string   `bson:"bin_path"               json:"bin_path"`
	Enabled      bool     `bson:"enabled"                json:"enabled"`
	DownloadPath string   `bson:"download_path"          json:"download_path"`
}

func (Install) TableName() string {
	return "install"
}
