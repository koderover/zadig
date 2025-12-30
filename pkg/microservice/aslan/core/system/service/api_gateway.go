/*
 * Copyright 2025 The KodeRover Authors.
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
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func ListApiGateway(log *zap.SugaredLogger) ([]*models.ApiGateway, error) {
	list, err := mongodb.NewApiGatewayColl().List()
	if err != nil {
		log.Errorf("List api gateway error: %v", err)
		return nil, e.ErrListApiGateway.AddErr(err)
	}
	return list, nil
}

func CreateApiGateway(apiGateway *models.ApiGateway, log *zap.SugaredLogger) error {
	if err := mongodb.NewApiGatewayColl().Create(apiGateway); err != nil {
		log.Errorf("Create api gateway error: %v", err)
		return e.ErrCreateApiGateway.AddErr(err)
	}
	return nil
}

func UpdateApiGateway(idHex string, apiGateway *models.ApiGateway, log *zap.SugaredLogger) error {
	if err := mongodb.NewApiGatewayColl().UpdateByID(idHex, apiGateway); err != nil {
		log.Errorf("Update api gateway error: %v", err)
		return e.ErrUpdateApiGateway.AddErr(err)
	}
	return nil
}

func DeleteApiGateway(idHex string, log *zap.SugaredLogger) error {
	if err := mongodb.NewApiGatewayColl().DeleteByID(idHex); err != nil {
		log.Errorf("Delete api gateway error: %v", err)
		return e.ErrDeleteApiGateway.AddErr(err)
	}
	return nil
}

// ValidateApiGateway validates the api gateway connection
// TODO: Implement the actual validation logic
func ValidateApiGateway(apiGateway *models.ApiGateway, log *zap.SugaredLogger) error {
	// Placeholder for validation logic
	// The user will implement the actual validation
	return nil
}

