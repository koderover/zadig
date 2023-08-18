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
	"fmt"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
)

const (
	VerbUpdateName      = "update_name"
	VerbUpdateDesc      = "update_desc"
	VerbUpdateTimeRange = "update_time_range"
	VerbUpdatePrincipal = "update_principal"
)

type PlanUpdater interface {
	Update(plan *models.ReleasePlan) error
	Lint() error
}

func NewPlanUpdater(args *UpdateReleasePlanArgs) (PlanUpdater, error) {
	switch args.Verb {
	case VerbUpdateName:
		return NewNameUpdater(args)
	case VerbUpdateDesc:
		return NewDescUpdater(args)
	case VerbUpdateTimeRange:
		return NewTimeRangeUpdater(args)
	case VerbUpdatePrincipal:
		return NewPrincipalUpdater(args)
	default:
		return nil, fmt.Errorf("invalid verb: %s", args.Verb)
	}
}

type NameUpdater struct {
	Name string `json:"name"`
}

func NewNameUpdater(args *UpdateReleasePlanArgs) (*NameUpdater, error) {
	var updater NameUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *NameUpdater) Update(plan *models.ReleasePlan) error {
	plan.Name = u.Name
	return nil
}

func (u *NameUpdater) Lint() error {
	if u.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	return nil
}
