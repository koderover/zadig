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

func CodeHostListPRs(codeHostID int, projectName, namespace, targetBr string, key string, page, perPage int, log *zap.SugaredLogger) ([]*client.PullRequest, error) {
	codehostClient, err := open.OpenClient(codeHostID, log)
	if err != nil {
		log.Errorf("open client err:%s", err)
		return nil, err
	}
	prs, err := codehostClient.ListPrs(client.ListOpt{
		Namespace:   namespace,
		ProjectName: projectName,
		Key:         key,
		Page:        page,
		PerPage:     perPage,
		TargeBr:     targetBr,
	})
	if err != nil {
		log.Errorf("list prs err:%s", err)
		return nil, err
	}
	return prs, nil
}
