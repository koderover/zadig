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

package service

import (
	"net/http"
	"net/url"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"
	"github.com/koderover/zadig/pkg/microservice/picket/config"
)

//DownloadKubeConfig user download kube config file which has permission to read or edit namespaces he has permission to
//query the opa service to get the project lists by pass through *rules parameter action
func GetKubeConfig(header http.Header, qs url.Values, logger *zap.SugaredLogger) ([]byte, error) {
	readEnvRules := []*rule{{
		method:   "GET",
		endpoint: "/api/aslan/environment/environments",
	}}
	editEnvRules := []*rule{{
		method:   "PUT",
		endpoint: "/api/aslan/environment/environments/?*",
	}, {
		method:   "POST",
		endpoint: "/api/aslan/environment/environments/?*/services/?*/restart",
	},
	}
	projectsEnvCanView, err := getAllowedProjects(header, readEnvRules, config.OR, logger)
	if err != nil {
		logger.Errorf("Failed to get allowed project names, err: %s", err)
		return nil, err
	}

	for _, name := range projectsEnvCanView {
		qs.Add("projectsEnvCanView", name)
	}

	projectsEnvCanEdit, err := getAllowedProjects(header, editEnvRules, config.OR, logger)
	if err != nil {
		logger.Errorf("Failed to get allowed project names, err: %s", err)
		return nil, err
	}

	for _, name := range projectsEnvCanEdit {
		qs.Add("projectsEnvCanEdit", name)
	}

	return aslan.New().DownloadKubeConfig(header, qs)
}
