/*
 * Copyright 2026 The KodeRover Authors.
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
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

func createReleasePlanVersion(planID string, version int64, baseSnapshot, snapshot interface{}, operator, account, sectionKey, sectionName, verb string) error {
	return mongodb.NewReleasePlanVersionColl().Create(&models.ReleasePlanVersion{
		PlanID:       planID,
		Version:      version,
		Operator:     operator,
		Account:      account,
		SectionKey:   sectionKey,
		SectionName:  sectionName,
		Verb:         verb,
		BaseSnapshot: sanitizeReleasePlanValue(baseSnapshot),
		Snapshot:     sanitizeReleasePlanValue(snapshot),
		CreatedAt:    time.Now().Unix(),
	})
}
