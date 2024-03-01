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
	"errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/grafana"
	"github.com/koderover/zadig/v2/pkg/tool/guanceyun"
)

func ListObservability(_type string, isAdmin bool) ([]*models.Observability, error) {
	resp, err := mongodb.NewObservabilityColl().List(context.Background(), _type)
	if err != nil {
		return nil, e.ErrListObservabilityIntegration.AddErr(err)
	}
	if !isAdmin {
		for _, v := range resp {
			v.ApiKey = ""
			v.GrafanaToken = ""
		}
	}
	return resp, nil
}

func CreateObservability(args *models.Observability) error {
	if err := mongodb.NewObservabilityColl().Create(context.Background(), args); err != nil {
		return e.ErrCreateObservabilityIntegration.AddErr(err)
	}
	return nil
}

func UpdateObservability(id string, args *models.Observability) error {
	if err := mongodb.NewObservabilityColl().Update(context.Background(), id, args); err != nil {
		return e.ErrUpdateObservabilityIntegration.AddErr(err)
	}
	return nil
}

func DeleteObservability(id string) error {
	if err := mongodb.NewObservabilityColl().DeleteByID(context.Background(), id); err != nil {
		return e.ErrDeleteObservabilityIntegration.AddErr(err)
	}
	return nil
}

func ValidateObservability(args *models.Observability) error {
	switch args.Type {
	case config.ObservabilityTypeGuanceyun:
		return validateGuanceyun(args)
	case config.ObservabilityTypeGrafana:
		return validateGrafana(args)
	default:
		return errors.New("invalid observability type")
	}
}

func validateGuanceyun(args *models.Observability) error {
	_, _, err := guanceyun.NewClient(args.Host, args.ApiKey).ListMonitor("", 1, 1)
	return err
}

func validateGrafana(args *models.Observability) error {
	_, err := grafana.NewClient(args.Host, args.GrafanaToken).ListAlertInstance()
	return err
}
