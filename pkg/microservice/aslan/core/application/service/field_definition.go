/*
Copyright 2025 The KodeRover Authors.

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
	"context"
	"strings"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func CreateFieldDefinition(def *commonmodels.ApplicationFieldDefinition, logger *zap.SugaredLogger) (*commonmodels.ApplicationFieldDefinition, error) {
	if def == nil {
		return nil, e.ErrInvalidParam.AddDesc("empty body")
	}
	// normalize
	def.Key = strings.TrimSpace(def.Key)
	if err := def.Validate(); err != nil {
		return nil, e.ErrInvalidParam.AddDesc(err.Error())
	}
	oid, err := commonrepo.NewApplicationFieldDefinitionColl().Create(context.Background(), def)
	if err != nil {
		return nil, err
	}
	def.ID = oid
	return def, nil
}

func ListFieldDefinitions(logger *zap.SugaredLogger) ([]*commonmodels.ApplicationFieldDefinition, error) {
	return commonrepo.NewApplicationFieldDefinitionColl().List(context.Background())
}

func UpdateFieldDefinition(key string, def *commonmodels.ApplicationFieldDefinition, logger *zap.SugaredLogger) error {
	if def == nil {
		return e.ErrInvalidParam.AddDesc("empty body")
	}
	if def.Key != "" && def.Key != key {
		return e.ErrInvalidParam.AddDesc("key is immutable")
	}
	def.Key = key
	if err := def.Validate(); err != nil {
		return e.ErrInvalidParam.AddDesc(err.Error())
	}
	return commonrepo.NewApplicationFieldDefinitionColl().UpdateByKey(context.Background(), key, def)
}

func DeleteFieldDefinition(key string, logger *zap.SugaredLogger) error {
	return commonrepo.NewApplicationFieldDefinitionColl().DeleteByKey(context.Background(), key)
}
