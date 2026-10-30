/*
Copyright 2026 The KodeRover Authors.

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

package workflow

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/apisix"
)

type ApisixCompatibilityError struct {
	Code         string                      `json:"code"`
	Message      string                      `json:"message"`
	ApisixID     string                      `json:"apisix_id"`
	TaskIndex    int                         `json:"task_index"`
	ResourceType config.ApisixItemType       `json:"resource_type"`
	ResourceName string                      `json:"resource_name"`
	ResourceID   string                      `json:"resource_id"`
	Fields       []apisix.ProtoOptionalField `json:"fields"`
}

type ApisixCompatibilityCheckResponse struct {
	Valid  bool                        `json:"valid"`
	Errors []*ApisixCompatibilityError `json:"errors"`
}

// CheckApisixJobCompatibility checks for known unsupported proto3 optional
// declarations. The frontend must block execution when Valid is false.
// This does not contact the gateway or perform full configuration validation.
func CheckApisixJobCompatibility(spec *commonmodels.ApisixJobSpec) *ApisixCompatibilityCheckResponse {
	resp := &ApisixCompatibilityCheckResponse{Valid: true, Errors: make([]*ApisixCompatibilityError, 0)}
	if spec == nil {
		return resp
	}
	for index, task := range spec.Tasks {
		if task == nil || task.Type != config.ApisixItemTypeProto ||
			(task.Action != config.ApisixActionTypeCreate && task.Action != config.ApisixActionTypeUpdate) {
			continue
		}
		proto := new(apisix.Proto)
		if err := commonmodels.IToi(task.Spec, proto); err != nil {
			continue
		}
		fields := apisix.FindProto3OptionalFields(proto.Content)
		if len(fields) == 0 {
			continue
		}
		name := proto.Name
		if name == "" {
			name = proto.ID
		}
		if name == "" {
			name = fmt.Sprintf("tasks[%d]", index)
		}
		resp.Errors = append(resp.Errors, &ApisixCompatibilityError{
			Code:         "apisix_proto3_optional",
			Message:      fmt.Sprintf("Proto 资源 %q 包含不支持的 proto3 optional 字段，请修改配置后再执行。", name),
			ApisixID:     spec.ApisixID,
			TaskIndex:    index,
			ResourceType: task.Type,
			ResourceName: proto.Name,
			ResourceID:   proto.ID,
			Fields:       fields,
		})
	}
	resp.Valid = len(resp.Errors) == 0
	return resp
}
