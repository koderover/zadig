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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

type CommitReleasePlanVersionArgs struct {
	SessionID  string `json:"session_id"`
	SectionKey string `json:"section_key"`
}

func createReleasePlanVersion(planID string, baseVersion, version int64, baseSnapshot, snapshot interface{}, operator, account, sectionKey, sectionName, verb string) error {
	return mongodb.NewReleasePlanVersionColl().Create(&models.ReleasePlanVersion{
		PlanID:       planID,
		BaseVersion:  baseVersion,
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

func CommitReleasePlanVersion(ctx context.Context, planID string, args *CommitReleasePlanVersionArgs, userID, operator, account string) (*models.ReleasePlanVersion, error) {
	if args == nil {
		return nil, errors.New("nil commit args")
	}
	if args.SessionID == "" {
		return nil, errors.New("empty session id")
	}
	approveLock := getLock(planID)
	approveLock.Lock()
	defer approveLock.Unlock()

	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		return nil, errors.Wrap(err, "get plan")
	}
	if err := validateReleasePlanEditingPlan(plan); err != nil {
		return nil, err
	}

	session, err := getReleasePlanEditingSession(planID, args.SessionID)
	if err != nil {
		return nil, errors.Wrap(err, "get editing session")
	}
	if session.UserID != "" && userID != "" && session.UserID != userID {
		return nil, errors.New("editing session does not belong to current user")
	}

	pending, err := mongodb.NewReleasePlanLogColl().CountPendingBySessionID(planID, args.SessionID)
	if err != nil {
		return nil, errors.Wrap(err, "count pending session logs")
	}
	if pending == 0 {
		return &models.ReleasePlanVersion{
			PlanID:     planID,
			Version:    plan.Version,
			Operator:   operator,
			Account:    account,
			SectionKey: args.SectionKey,
			CreatedAt:  time.Now().Unix(),
		}, nil
	}

	baseSnapshot, err := decodeReleasePlanVersionSnapshot(session.BaseSnapshot)
	if err != nil {
		return nil, errors.Wrap(err, "decode session snapshot")
	}

	fromVersion := session.BaseVersion
	if plan.Version == 0 {
		fromVersion, err = ensureReleasePlanBaselineVersion(ctx, planID, plan)
		if err != nil {
			return nil, errors.Wrap(err, "ensure baseline version")
		}
	}
	if fromVersion == 0 {
		fromVersion = plan.Version
	}

	currentVersion, err := mongodb.NewReleasePlanColl().IncrementVersionByID(ctx, planID)
	if err != nil {
		return nil, errors.Wrap(err, "increment plan version")
	}
	plan.Version = currentVersion

	currentSnapshot, err := buildReleasePlanVersionSnapshot(plan, args.SectionKey)
	if err != nil {
		return nil, errors.Wrap(err, "build release plan section snapshot")
	}

	if err := createReleasePlanVersion(planID, fromVersion, currentVersion, baseSnapshot, currentSnapshot, operator, account, args.SectionKey, releasePlanVersionSectionName(args.SectionKey, session.SectionName), "commit"); err != nil {
		return nil, errors.Wrap(err, "create committed version")
	}
	if err := mongodb.NewReleasePlanLogColl().FillVersionsBySessionID(planID, args.SessionID, fromVersion, currentVersion); err != nil {
		return nil, errors.Wrap(err, "fill log versions")
	}

	session.BaseVersion = currentVersion
	session.BaseSnapshot = encodeReleasePlanVersionSnapshot(currentSnapshot)
	if err := persistReleasePlanEditingSession(session); err != nil {
		return nil, errors.Wrap(err, "refresh editing session")
	}

	broadcastReleasePlanCollaboration(planID)
	return &models.ReleasePlanVersion{
		PlanID:     planID,
		Version:    currentVersion,
		Operator:   operator,
		Account:    account,
		SectionKey: args.SectionKey,
		CreatedAt:  time.Now().Unix(),
	}, nil
}
