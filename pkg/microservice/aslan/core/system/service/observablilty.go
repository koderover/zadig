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

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListObservability() ([]*models.Observability, error) {
	resp, err := mongodb.NewObservabilityColl().List(context.Background())
	if err != nil {
		return nil, e.ErrListObservabilityIntegration.AddErr(err)
	}
	return resp, nil
}

func UpdateObservability(id string, log *zap.SugaredLogger) error {
	if err := mongodb.NewObservabilityColl().Update(context.Background(), id); err != nil {
		log.Errorf("update observability error: %v", err)
		return e.ErrUpdateObservabilityIntegration.AddErr(err)
	}
	return nil
}
