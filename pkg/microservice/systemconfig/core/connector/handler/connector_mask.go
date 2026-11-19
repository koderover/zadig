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

package handler

import (
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/connector/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

var connectorSecretKeys = []string{"bindPW", "clientSecret"}

// canEditConnector reports whether the user may see connector secrets.
// Users with view permission only (e.g. global read-only) get masked secrets.
func canEditConnector(ctx *internalhandler.Context) bool {
	if ctx == nil || ctx.Resources == nil {
		return false
	}
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	if ctx.Resources.SystemActions == nil || ctx.Resources.SystemActions.SystemIntegration == nil {
		return false
	}
	actions := ctx.Resources.SystemActions.SystemIntegration
	return actions.Create || actions.Edit
}

func maskConnectorSecrets(connectors ...*service.Connector) {
	for _, connector := range connectors {
		if connector == nil {
			continue
		}
		config, ok := connector.Config.(map[string]interface{})
		if !ok {
			continue
		}
		for _, key := range connectorSecretKeys {
			if value, ok := config[key]; ok && value != nil && value != "" {
				config[key] = setting.MaskValue
			}
		}
	}
}
