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
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/util"
)

// The service can be deployed only in the following situations:
// 1. All general environments are deployable.
// 2. All base environments are deployable.
// 3. If the service has been deployed in the baseline environment, all sub-environments of the baseline environment
//    can deploy the service.
//    Otherwise, all sub-environments of the baseline environment cannot deploy the service.
func GetDeployableEnvs(svcName, projectName string) ([]string, error) {
	// 1. Get all general environments.
	envs0, err := getAllGeneralEnvs(projectName)
	if err != nil {
		return nil, err
	}

	// 2. Get all deployable environments in the context of environment sharing..
	envs1, err := getDeployableShareEnvs(svcName, projectName)
	if err != nil {
		return nil, err
	}

	envs0 = append(envs0, envs1...)

	return envs0, nil
}

func getAllGeneralEnvs(projectName string) ([]string, error) {
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:           projectName,
		ShareEnvEnable: util.GetBoolPointer(false),
	})
	if err != nil {
		return nil, err
	}

	envNames := make([]string, len(envs))
	for i, env := range envs {
		envNames[i] = env.EnvName
	}

	return envNames, nil
}

func getDeployableShareEnvs(svcName, projectName string) ([]string, error) {
	baseEnvs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:           projectName,
		ShareEnvEnable: util.GetBoolPointer(true),
		ShareEnvIsBase: util.GetBoolPointer(true),
	})
	if err != nil {
		return nil, err
	}

	ret := []string{}
	for _, baseEnv := range baseEnvs {
		ret = append(ret, baseEnv.EnvName)

		if !hasSvcInEnv(svcName, baseEnv) {
			continue
		}

		subEnvs, err := getSubEnvs(baseEnv.EnvName, projectName)
		if err != nil {
			return nil, err
		}

		ret = append(ret, subEnvs...)
	}

	return ret, nil
}

func getSubEnvs(baseEnvName, projectName string) ([]string, error) {
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:            projectName,
		ShareEnvEnable:  util.GetBoolPointer(true),
		ShareEnvIsBase:  util.GetBoolPointer(false),
		ShareEnvBaseEnv: util.GetStrPointer(baseEnvName),
	})
	if err != nil {
		return nil, err
	}

	envNames := make([]string, len(envs))
	for i, env := range envs {
		envNames[i] = env.EnvName
	}

	return envNames, nil
}

func hasSvcInEnv(svcName string, env *commonmodels.Product) bool {
	for _, svcGroup := range env.Services {
		for _, svc := range svcGroup {
			if svc.ServiceName == svcName {
				return true
			}
		}
	}

	return false
}
