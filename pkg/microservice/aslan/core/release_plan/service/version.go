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
	"context"
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

var (
	updateReleasePlanDocument = func(ctx context.Context, planID string, plan *models.ReleasePlan) error {
		return mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan)
	}
	createReleasePlanVersionDocument = func(ctx context.Context, version *models.ReleasePlanVersion) error {
		return mongodb.NewReleasePlanVersionColl().CreateWithCtx(ctx, version)
	}
	deleteReleasePlanVersionDocument = func(ctx context.Context, planID string, version int64) error {
		return mongodb.NewReleasePlanVersionColl().DeleteWithCtx(ctx, planID, version)
	}
)

func createReleasePlanVersion(planID string, version int64, snapshot interface{}, operator, account, sectionKey, sectionName, verb string) error {
	return createReleasePlanVersionWithBaseSnapshotCtx(context.Background(), planID, version, 0, nil, snapshot, operator, account, sectionKey, sectionName, verb)
}

func createReleasePlanVersionWithBaseSnapshot(planID string, version, previousVersion int64, baseSnapshot, snapshot interface{}, operator, account, sectionKey, sectionName, verb string) error {
	return createReleasePlanVersionWithBaseSnapshotCtx(context.Background(), planID, version, previousVersion, baseSnapshot, snapshot, operator, account, sectionKey, sectionName, verb)
}

func createReleasePlanVersionWithBaseSnapshotCtx(ctx context.Context, planID string, version, previousVersion int64, baseSnapshot, snapshot interface{}, operator, account, sectionKey, sectionName, verb string) error {
	return createReleasePlanVersionDocument(ctx, newReleasePlanVersionDocument(planID, version, previousVersion, baseSnapshot, snapshot, operator, account, sectionKey, sectionName, verb))
}

func newReleasePlanVersionDocument(planID string, version, previousVersion int64, baseSnapshot, snapshot interface{}, operator, account, sectionKey, sectionName, verb string) *models.ReleasePlanVersion {
	return &models.ReleasePlanVersion{
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
	}
}

func persistReleasePlanWithVersion(ctx context.Context, planID string, plan *models.ReleasePlan, versionDoc *models.ReleasePlanVersion) error {
	if plan == nil {
		return errors.New("nil release plan")
	}
	if versionDoc == nil {
		return errors.New("nil release plan version")
	}

	if config.EnableTransaction() {
		session, deferSession, err := mongotool.SessionWithTransaction(ctx)
		if err != nil {
			return errors.Wrap(err, "start release plan transaction")
		}

		var retErr error
		defer func() {
			deferSession(retErr)
		}()

		sessionCtx := mongotool.SessionContext(ctx, session)
		if err := updateReleasePlanDocument(sessionCtx, planID, plan); err != nil {
			retErr = errors.Wrap(err, "update plan")
			return retErr
		}
		if err := createReleasePlanVersionDocument(sessionCtx, versionDoc); err != nil {
			retErr = errors.Wrap(err, "create release plan version")
			return retErr
		}
		if err := mongotool.CommitTransaction(session); err != nil {
			retErr = errors.Wrap(err, "commit release plan transaction")
			return retErr
		}
		return nil
	}

	if err := createReleasePlanVersionDocument(ctx, versionDoc); err != nil {
		return errors.Wrap(err, "create release plan version")
	}
	if err := updateReleasePlanDocument(ctx, planID, plan); err != nil {
		cleanupErr := deleteReleasePlanVersionDocument(ctx, planID, versionDoc.Version)
		if cleanupErr != nil {
			return errors.Wrapf(err, "update plan; cleanup release plan version error: %v", cleanupErr)
		}
		return errors.Wrap(err, "update plan")
	}
	return nil
}

func shouldBuildReleasePlanVersionBaseSnapshot(planID, sectionKey string, version int64, verb UpdateReleasePlanVerb) (bool, int64, error) {
	switch verb {
	case VerbDeleteReleaseJob, VerbDeleteApproval, VerbReorderReleaseJob:
		return true, version - 1, nil
	default:
		previousVersion, err := previousComparableReleasePlanVersion(planID, sectionKey, version)
		if err != nil {
			return false, 0, err
		}
		if previousVersion == 0 {
			return true, version - 1, nil
		}
		return false, previousVersion, nil
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

func shouldBuildReleasePlanWorkflowDisplayBaseSnapshot(planID, sectionKey string, previousVersion int64, currentSnapshot interface{}) (bool, error) {
	if previousVersion == 0 || releasePlanVersionSectionGroupType(sectionKey) != "job" || !isReleasePlanWorkflowJobSnapshot(currentSnapshot) {
		return false, nil
	}

	previous, err := mongodb.NewReleasePlanVersionColl().Get(planID, previousVersion)
	if err != nil {
		return false, err
	}
	previousSnapshot := comparableReleasePlanVersionSnapshot(previous, sectionKey)
	return isIncompleteReleasePlanWorkflowDisplaySnapshot(previousSnapshot, currentSnapshot), nil
}

func isIncompleteReleasePlanWorkflowDisplaySnapshot(previousSnapshot, currentSnapshot interface{}) bool {
	previousSpec, ok := getMapField(releasePlanVersionDiffJobSpec(previousSnapshot))
	if !ok {
		return false
	}
	currentSpec, ok := getMapField(releasePlanVersionDiffJobSpec(currentSnapshot))
	if !ok {
		return false
	}

	return hasMissingReleasePlanWorkflowDisplayFields(currentSpec, previousSpec)
}

func hasMissingReleasePlanWorkflowDisplayFields(reference, candidate interface{}) bool {
	switch typedReference := reference.(type) {
	case map[string]interface{}:
		typedCandidate, ok := candidate.(map[string]interface{})
		if !ok {
			return true
		}
		for key, referenceValue := range typedReference {
			candidateValue, exists := typedCandidate[key]
			if !exists {
				return true
			}
			if hasMissingReleasePlanWorkflowDisplayFields(referenceValue, candidateValue) {
				return true
			}
		}
	case []interface{}:
		typedCandidate, ok := candidate.([]interface{})
		if !ok {
			return true
		}
		limit := len(typedReference)
		if len(typedCandidate) < limit {
			limit = len(typedCandidate)
		}
		for idx := 0; idx < limit; idx++ {
			if hasMissingReleasePlanWorkflowDisplayFields(typedReference[idx], typedCandidate[idx]) {
				return true
			}
		}
	}
	return false
}
