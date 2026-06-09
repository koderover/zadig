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

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

func createReleasePlanVersion(planID string, version int64, snapshot interface{}, operator, account, sectionKey, sectionName, verb string) error {
	return createReleasePlanVersionWithBaseSnapshot(planID, version, nil, snapshot, operator, account, sectionKey, sectionName, verb)
}

func createReleasePlanVersionWithBaseSnapshot(planID string, version int64, baseSnapshot, snapshot interface{}, operator, account, sectionKey, sectionName, verb string) error {
	var previousVersion int64
	if baseSnapshot != nil {
		previousVersion = version - 1
	} else {
		var err error
		previousVersion, err = previousComparableReleasePlanVersion(planID, sectionKey, version)
		if err != nil {
			return err
		}
	}

	return mongodb.NewReleasePlanVersionColl().Create(&models.ReleasePlanVersion{
		PlanID:          planID,
		Version:         version,
		PreviousVersion: previousVersion,
		Operator:        operator,
		Account:         account,
		SectionKey:      sectionKey,
		SectionName:     sectionName,
		SectionType:     releasePlanVersionSectionGroupType(sectionKey),
		Verb:            verb,
		BaseSnapshot:    sanitizeReleasePlanValue(baseSnapshot),
		Snapshot:        sanitizeReleasePlanValue(snapshot),
		CreatedAt:       time.Now().Unix(),
	})
}

func shouldBuildReleasePlanVersionBaseSnapshot(verb UpdateReleasePlanVerb) bool {
	switch verb {
	case VerbDeleteReleaseJob, VerbDeleteApproval, VerbReorderReleaseJob:
		return true
	default:
		return false
	}
}

func previousComparableReleasePlanVersion(planID, sectionKey string, beforeVersion int64) (int64, error) {
	sectionKeys := []string{sectionKey}
	if sectionKey != releasePlanVersionSectionPlan {
		sectionKeys = append(sectionKeys, releasePlanVersionSectionPlan)
	}

	previous, err := mongodb.NewReleasePlanVersionColl().GetLatestBySectionsBefore(planID, sectionKeys, beforeVersion)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, nil
		}
		return 0, err
	}
	return previous.Version, nil
}
