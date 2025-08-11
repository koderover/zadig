/*
 * Copyright 2024 The KodeRover Authors.
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

package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/util"
	"go.mongodb.org/mongo-driver/bson"
)

func init() {
	upgradepath.RegisterHandler("3.2.0", "3.2.1", V320ToV321)
	upgradepath.RegisterHandler("3.2.1", "3.2.0", V321ToV320)
}

func V320ToV321() error {
	ctx := handler.NewBackgroupContext()

	ctx.Logger.Infof("-------- start migrate release plan cronjob --------")
	err := migrateReleasePlanCron(ctx)
	if err != nil {
		err = fmt.Errorf("failed to migrate release plan cronjob, error: %w", err)
		ctx.Logger.Error(err)
		return err
	}

	return nil
}

func V321ToV320() error {
	return nil
}

func migrateReleasePlanCron(ctx *handler.Context) error {
	// delete all release plan cronjob first
	_, err := commonrepo.NewCronjobColl().DeleteMany(ctx,
		bson.M{"type": "release_plan"},
	)
	if err != nil {
		return fmt.Errorf("failed to delete release plan cronjobs, error: %w", err)
	}

	cursor, err := commonrepo.NewReleasePlanColl().ListByCursor()
	if err != nil {
		return fmt.Errorf("failed to list release plans, error: %w", err)
	}
	for cursor.Next(context.Background()) {
		var releasePlan models.ReleasePlan
		if err := cursor.Decode(&releasePlan); err != nil {
			return err
		}

		if releasePlan.ScheduleExecuteTime != 0 && releasePlan.Status == config.ReleasePlanStatusExecuting {
			if time.Unix(releasePlan.ScheduleExecuteTime, 0).After(time.Now()) {
				releasePlanCronName := util.GetReleasePlanCronName(releasePlan.ID.Hex(), releasePlan.Name, releasePlan.Index)
				cronjob := &commonmodels.Cronjob{
					Enabled:   true,
					Name:      releasePlanCronName,
					Type:      "release_plan",
					JobType:   string(config.UnixstampSchedule),
					UnixStamp: releasePlan.ScheduleExecuteTime,
					ReleasePlanArgs: &commonmodels.ReleasePlanArgs{
						ID:    releasePlan.ID.Hex(),
						Name:  releasePlan.Name,
						Index: releasePlan.Index,
					},
				}
				if err := commonrepo.NewCronjobColl().Upsert(cronjob); err != nil {
					return fmt.Errorf("failed to create new release plan schedule job, error: %w", err)
				}
			}
		}
	}

	return nil
}
