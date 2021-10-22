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

import "strconv"

type Connector struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	Name            string `json:"name"`
	ResourceVersion string `json:"resource_version"`
	Config          string `json:"config"`
}

func (m *Connector) IncreaseResourceVersion() {
	rv, _ := strconv.Atoi(m.ResourceVersion)
	m.ResourceVersion = strconv.Itoa(rv + 1)
}

// TableName sets the insert table name for this struct type
func (Connector) TableName() string {
	return "connector"
}
