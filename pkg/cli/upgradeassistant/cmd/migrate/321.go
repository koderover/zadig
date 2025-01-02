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
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	"go.mongodb.org/mongo-driver/bson"
)

func init() {
	upgradepath.RegisterHandler("3.2.0", "3.2.1", V320ToV321)
	upgradepath.RegisterHandler("3.2.1", "3.2.0", V321ToV320)
}

func V320ToV321() error {
	ctx := handler.NewBackgroupContext()
	ctx.Logger.Infof("-------- start 321 ua --------")

	return nil
}

func V321ToV320() error {
	return nil
}

func fixReleasePlanCron(ctx *handler.Context) error {
	// disable all release plan cronjob first
	updateResult, err := commonrepo.NewCronjobColl().UpdateMany(ctx,
		// bson.M{"cron": "0 8 1 1", "type": "release_plan", "enabled": true},
		bson.M{"type": "release_plan", "enabled": true},
		bson.M{"$set": bson.M{"enabled": false}})
	if err != nil {
		return err
	}
	if updateResult.ModifiedCount > 0 {
		ctx.Logger.Infof("update %d release plan cronjob", updateResult.ModifiedCount)
	}

	releasePlans, _, err := commonrepo.NewReleasePlanColl().ListByOptions(&commonrepo.ListReleasePlanOption{})
	if err != nil {
		return err
	}

	// create new cronjob for release plan if schedule time is after now
	for _, releasePlan := range releasePlans {
		if releasePlan.ScheduleExecuteTime != 0 {
			if time.Unix(releasePlan.ScheduleExecuteTime, 0).After(time.Now()) {
				cronjob := &commonmodels.Cronjob{
					Enabled:   true,
					Name:      releasePlan.Name,
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
					return err
				}
			}
		}
	}

	// sync to cron service

	return nil
}
