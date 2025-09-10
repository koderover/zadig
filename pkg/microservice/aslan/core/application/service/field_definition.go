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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
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
	// fields from create API is always custom
	def.Source = config.ApplicationFieldSourceCustom
	oid, err := commonrepo.NewApplicationFieldDefinitionColl().Create(context.Background(), def)
	if err != nil {
		return nil, err
	}
	def.ID = oid
	if def.Unique {
		if err := commonrepo.NewApplicationColl().CreateCustomFieldUniqueIndex(context.Background(), def.Key); err != nil {
			logger.Warnf("failed to create unique index for custom field %s: %v", def.Key, err)
		}
	}
	return def, nil
}

func ListFieldDefinitions(logger *zap.SugaredLogger) ([]*commonmodels.ApplicationFieldDefinition, error) {
	return commonrepo.NewApplicationFieldDefinitionColl().List(context.Background())
}

func UpdateFieldDefinition(id string, def *commonmodels.ApplicationFieldDefinition, logger *zap.SugaredLogger) error {
	if def == nil {
		return e.ErrInvalidParam.AddDesc("empty body")
	}

	if err := def.Validate(); err != nil {
		return e.ErrInvalidParam.AddDesc(err.Error())
	}

	return commonrepo.NewApplicationFieldDefinitionColl().UpdateByID(context.Background(), id, def)
}

func DeleteFieldDefinition(id string, logger *zap.SugaredLogger) error {
	// best-effort drop unique index if exists
	defColl := commonrepo.NewApplicationFieldDefinitionColl()
	// we don't have a direct lookup by id; attempt listing and find by id
	defs, _ := defColl.List(context.Background())
	var key string
	for _, d := range defs {
		if d.ID.Hex() == id {
			key = d.Key
			break
		}
	}
	if key != "" {
		_ = commonrepo.NewApplicationColl().UnsetCustomFieldForAll(context.Background(), key)
		_ = commonrepo.NewApplicationColl().DropCustomFieldUniqueIndex(context.Background(), key)
	}
	return defColl.DeleteByID(context.Background(), id)
}
