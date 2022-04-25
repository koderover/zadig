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
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/open"
)

func CodeHostListProjects(codeHostID int, namespace, namespaceType string, page, perPage int, keyword string, log *zap.SugaredLogger) ([]*client.Project, error) {
	codehostClient, err := open.OpenClient(codeHostID, log)
	if err != nil {
		log.Errorf("open client err:%s", err)
		return nil, err
	}
	projects, err := codehostClient.ListProjects(client.ListOpt{
		Namespace:     namespace,
		NamespaceType: namespaceType,
		Key:           keyword,
		Page:          page,
		PerPage:       perPage,
	})
	if err != nil {
		log.Errorf("list projects err:%s", err)
		return nil, err
	}
	return projects, nil
}
