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

import commonmodes "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"

type Chart struct {
	Name       string                  `bson:"name"         json:"name"`
	Source     string                  `bson:"source"       json:"source"`
	Owner      string                  `bson:"owner"        json:"owner"`
	Repo       string                  `bson:"repo"         json:"repo"`
	Path       string                  `bson:"path"         json:"path"`
	Branch     string                  `bson:"branch"         json:"branch"`
	CodeHostID int                     `bson:"codehost_id"  json:"codeHostID"`
	Revision   int64                   `bson:"revision"     json:"revision"`
	Variables  []*commonmodes.Variable `bson:"variables"     json:"variables"`
	Sha1       string                  `bson:"sha1"         json:"sha1"`
}

func (chart *Chart) GetVariableMap() map[string]*commonmodes.Variable {
	if len(chart.Variables) == 0 {
		return nil
	}
	ret := make(map[string]*commonmodes.Variable)
	for _, v := range chart.Variables {
		ret[v.Key] = v
	}
	return ret
}

func (Chart) TableName() string {
	return "chart_template"
}
