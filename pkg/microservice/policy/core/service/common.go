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

package service

import (
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
)

type Rule struct {
	Verbs            []string                `json:"verbs"`
	Resources        []string                `json:"resources"`
	Kind             string                  `json:"kind"`
	MatchAttributes  []models.MatchAttribute `json:"match_attributes"`
	RelatedResources []string                `json:"related_resources"`
}

const SystemScope = "*"
const PresetScope = ""
