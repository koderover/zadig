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

package service

import (
	"fmt"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// ResolveEnvServiceIsHelmChart reports whether serviceName refers to a chart-only release in the env.
// Zadig services are keyed by service name and chart-only releases by release name, so the same name
// may match both; the caller must then specify isHelmChart explicitly.
func ResolveEnvServiceIsHelmChart(projectKey, envName, serviceName string, production bool) (bool, error) {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       projectKey,
		EnvName:    envName,
		Production: &production,
	})
	if err != nil {
		return false, e.ErrGetEnv.AddErr(fmt.Errorf("failed to find env %s/%s, production %v: %w", projectKey, envName, production, err))
	}
	_, isService := env.GetServiceMap()[serviceName]
	_, isChart := env.GetChartServiceMap()[serviceName]
	if isService && isChart {
		return false, e.ErrInvalidParam.AddDesc(fmt.Sprintf("%s matches both a service and a chart release in env %s, specify isHelmChart", serviceName, envName))
	}
	return isChart, nil
}
