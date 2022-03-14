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
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CodeHostListTags(codeHostID int, projectName string, namespace string, key string, page int, perPage int, log *zap.SugaredLogger) ([]*client.Tag, error) {
	ch, err := systemconfig.New().GetCodeHost(codeHostID)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListTags.AddDesc("git client is nil")
	}

	codehostClient, err := open.OpenClient(ch.ID, log)
	if err != nil {
		return nil, err
	}
	return codehostClient.ListTags(client.ListOpt{
		Namespace:   namespace,
		ProjectName: projectName,
		Key:         key,
		Page:        page,
		PerPage:     page,
	})
}
