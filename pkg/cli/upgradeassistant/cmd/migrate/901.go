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

package migrate

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	workflowController "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func init() {
	upgradepath.RegisterHandler("9.0.0", "9.0.1", V900ToV901)
	upgradepath.RegisterHandler("9.0.1", "9.0.0", V901ToV900)
}

func V900ToV901() error {
	ctx := internalhandler.NewBackgroupContext()

	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}

	defer func() {
		updateMigrationError(migrationInfo.ID, err)
	}()

	cursor, err := commonrepo.NewReleasePlanColl().ListByCursor()
	if err != nil {
		return fmt.Errorf("failed to list release plans, error: %w", err)
	}

	count := 0
	for cursor.Next(ctx) {
		var releasePlan models.ReleasePlan
		if err := cursor.Decode(&releasePlan); err != nil {
			return fmt.Errorf("failed to decode release plan, err: %v", err)
		}

		for _, job := range releasePlan.Jobs {
			if job.Type == config.JobWorkflow {
				releaseJobSpec := new(models.WorkflowReleaseJobSpec)
				err = models.IToi(job.Spec, releaseJobSpec)
				if err != nil {
					return fmt.Errorf("failed to decode release job spec, release plan name: %s, err: %v", releasePlan.Name, err)
				}

				if releaseJobSpec.Workflow == nil {
					log.Warnf("release plan %s job's workflow spec is nil", releasePlan.Name)
					continue
				}

				controller := workflowController.CreateWorkflowController(releaseJobSpec.Workflow)
				err = controller.ClearOptions()
				if err != nil {
					return fmt.Errorf("failed to clear options, release plan name: %s, workflow name: %s, err", releasePlan.Name, releaseJobSpec.Workflow.Name)
				}

				job.Spec = releaseJobSpec
			}
		}

		err = commonrepo.NewReleasePlanColl().UpdateByID(ctx, releasePlan.ID.Hex(), &releasePlan)
		if err != nil {
			return fmt.Errorf("failed update release plan, name: %s, err: %v", releasePlan.Name, err)
		}

		count++
	}

	log.Infof("finished clear release plan options data, updated %v release plans", count)

	return nil
}

func V901ToV900() error {
	return nil
}
