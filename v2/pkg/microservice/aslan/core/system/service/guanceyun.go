/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"context"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/guanceyun"
)

func ListGuanceyunMonitor(id, search string) ([]guanceyun.MonitorContent, error) {
	info, err := mongodb.NewObservabilityColl().GetByID(context.Background(), id)
	if err != nil {
		return nil, errors.Wrapf(err, "get observability info %s failed", id)
	}

	contents, err := guanceyun.NewClient(info.Host, info.ApiKey).ListAllMonitor(search)
	if err != nil {
		return nil, errors.Wrapf(err, "list guanceyun monitor failed")
	}

	return contents, nil
}
