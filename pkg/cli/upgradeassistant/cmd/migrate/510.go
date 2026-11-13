/*
Copyright 2026 The KodeRover Authors.

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
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	internalmodels "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/config"
	aslanconfig "github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	connectormodels "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/repository/models"
	gormtool "github.com/koderover/zadig/v2/pkg/tool/gorm"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"gorm.io/gorm"
)

func init() {
	upgradepath.RegisterHandler("5.0.0", "5.1.0", V500ToV510)
	upgradepath.RegisterHandler("5.1.0", "5.0.0", V510ToV500)
}

func V500ToV510() error {
	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}
	defer func() {
		updateMigrationError(migrationInfo.ID, err)
	}()

	err = migrateLarkPreferredUsernameClaim510(migrationInfo)
	if err != nil {
		return err
	}

	err = migrateDockerBuildProxy510(migrationInfo)
	if err != nil {
		return err
	}

	err = migrateWorkflowJobPolicies510(migrationInfo)
	return err
}

// Backfill policies in place so historical snapshots and workflow deltas keep
// their version IDs, and fields unknown to the current models are preserved.
func migrateWorkflowJobPolicies510(migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration510WorkflowJobPolicies {
		return nil
	}
	var migrationErrors []error
	if err := backfillWorkflowJobPolicies510(commonrepo.NewWorkflowV4TemplateColl().Collection); err != nil {
		migrationErrors = append(migrationErrors, err)
	}
	if err := backfillWorkflowTemplateVersionJobPolicies510(); err != nil {
		migrationErrors = append(migrationErrors, err)
	}
	if err := backfillWorkflowJobPolicies510(commonrepo.NewWorkflowV4Coll().Collection); err != nil {
		migrationErrors = append(migrationErrors, err)
	}
	if err := errors.Join(migrationErrors...); err != nil {
		return err
	}
	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration510WorkflowJobPolicies): true,
	})
}

func backfillWorkflowJobPolicies510(coll *mongo.Collection) error {
	ctx := context.Background()
	filter := bson.M{"stages": bson.M{"$elemMatch": bson.M{
		"jobs": bson.M{"$elemMatch": bson.M{"$or": bson.A{
			bson.M{"execute_policy": nil},
			bson.M{"execute_policy.type": nil},
			bson.M{"execute_policy.match_rule": bson.M{"$in": bson.A{nil, ""}}},
			bson.M{"execute_policy.rules": nil},
			bson.M{"error_policy": nil},
			bson.M{"error_policy.policy": bson.M{"$in": bson.A{nil, ""}}},
			bson.M{"error_policy.maximum_retry": nil},
			bson.M{"error_policy.approval_users": nil},
		}}},
	}}}
	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list jobs without policies in %s: %w", coll.Name(), err)
	}
	defer cursor.Close(ctx)

	var modified int64
	var failures []error
	for cursor.Next(ctx) {
		var document struct {
			ID     primitive.ObjectID            `bson:"_id"`
			Stages []*commonmodels.WorkflowStage `bson:"stages"`
		}
		if err := cursor.Decode(&document); err != nil {
			err = fmt.Errorf("failed to decode document %s in %s: %w", cursor.Current.Lookup("_id").String(), coll.Name(), err)
			log.Error(err)
			failures = append(failures, err)
			continue
		}
		updates := bson.M{}
		for stageIndex, stage := range document.Stages {
			if stage == nil {
				continue
			}
			for jobIndex, job := range stage.Jobs {
				if job == nil {
					continue
				}
				rawJob := cursor.Current.Lookup("stages", strconv.Itoa(stageIndex), "jobs", strconv.Itoa(jobIndex)).Document()
				backfillJobPolicies510(job, rawJob, fmt.Sprintf("stages.%d.jobs.%d", stageIndex, jobIndex), updates)
			}
		}
		if len(updates) == 0 {
			continue
		}
		if coll.Name() == (commonmodels.WorkflowV4{}).TableName() {
			workflow := new(commonmodels.WorkflowV4)
			if err := cursor.Decode(workflow); err != nil {
				err = fmt.Errorf("failed to decode workflow %s in %s: %w", document.ID.Hex(), coll.Name(), err)
				log.Error(err)
				failures = append(failures, err)
				continue
			}
			workflow.Stages = document.Stages
			workflow.UpdateHash()
			updates["hash"] = workflow.Hash
		}
		res, err := coll.UpdateOne(ctx, bson.M{"_id": document.ID}, bson.M{"$set": updates})
		if err != nil {
			err = fmt.Errorf("failed to backfill job policies in %s for %s: %w", coll.Name(), document.ID.Hex(), err)
			log.Error(err)
			failures = append(failures, err)
			continue
		}
		modified += res.ModifiedCount
	}
	if err := cursor.Err(); err != nil {
		return errors.Join(append(failures, fmt.Errorf("failed to iterate %s: %w", coll.Name(), err))...)
	}
	log.Infof("migration 5.1.0: backfilled job policies for %d documents in %s, failed: %d", modified, coll.Name(), len(failures))
	return errors.Join(failures...)
}

func backfillWorkflowTemplateVersionJobPolicies510() error {
	coll := commonrepo.NewWorkflowV4TemplateVersionColl().Collection
	ctx := context.Background()
	filter := bson.M{"snapshot.stages": bson.M{"$elemMatch": bson.M{
		"jobs": bson.M{"$elemMatch": bson.M{"$or": bson.A{
			bson.M{"execute_policy": nil},
			bson.M{"execute_policy.type": nil},
			bson.M{"execute_policy.match_rule": bson.M{"$in": bson.A{nil, ""}}},
			bson.M{"execute_policy.rules": nil},
			bson.M{"error_policy": nil},
			bson.M{"error_policy.policy": bson.M{"$in": bson.A{nil, ""}}},
			bson.M{"error_policy.maximum_retry": nil},
			bson.M{"error_policy.approval_users": nil},
		}}},
	}}}
	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list jobs without policies in %s: %w", coll.Name(), err)
	}
	defer cursor.Close(ctx)

	var modified int64
	var failures []error
	for cursor.Next(ctx) {
		var version commonmodels.WorkflowV4TemplateVersion
		if err := cursor.Decode(&version); err != nil {
			err = fmt.Errorf("failed to decode document %s in %s: %w", cursor.Current.Lookup("_id").String(), coll.Name(), err)
			log.Error(err)
			failures = append(failures, err)
			continue
		}
		if version.Snapshot == nil {
			continue
		}
		updates := bson.M{}
		for stageIndex, stage := range version.Snapshot.Stages {
			if stage == nil {
				continue
			}
			for jobIndex, job := range stage.Jobs {
				if job == nil {
					continue
				}
				rawJob := cursor.Current.Lookup("snapshot", "stages", strconv.Itoa(stageIndex), "jobs", strconv.Itoa(jobIndex)).Document()
				backfillJobPolicies510(job, rawJob, fmt.Sprintf("snapshot.stages.%d.jobs.%d", stageIndex, jobIndex), updates)
			}
		}
		if len(updates) == 0 {
			continue
		}
		snapshot, err := json.Marshal(version.Snapshot)
		if err != nil {
			err = fmt.Errorf("failed to marshal template snapshot %s in %s: %w", version.ID.Hex(), coll.Name(), err)
			log.Error(err)
			failures = append(failures, err)
			continue
		}
		updates["hash"] = fmt.Sprintf("%x", md5.Sum(snapshot))
		res, err := coll.UpdateOne(ctx, bson.M{"_id": version.ID}, bson.M{"$set": updates})
		if err != nil {
			err = fmt.Errorf("failed to backfill job policies in %s for %s: %w", coll.Name(), version.ID.Hex(), err)
			log.Error(err)
			failures = append(failures, err)
			continue
		}
		modified += res.ModifiedCount
	}
	if err := cursor.Err(); err != nil {
		return errors.Join(append(failures, fmt.Errorf("failed to iterate %s: %w", coll.Name(), err))...)
	}
	log.Infof("migration 5.1.0: backfilled job policies for %d documents in %s, failed: %d", modified, coll.Name(), len(failures))
	return errors.Join(failures...)
}

// Keep existing policy values and unknown fields; only fill absent/defaulted fields.
func backfillJobPolicies510(job *commonmodels.Job, rawJob bson.Raw, jobPath string, updates bson.M) {
	if job.ExecutePolicy == nil {
		job.ExecutePolicy = &commonmodels.JobExecutePolicy{
			MatchRule: aslanconfig.JobExecutePolicyMatchRuleAll,
			Rules:     []*commonmodels.JobExecuteRule{},
		}
		updates[jobPath+".execute_policy"] = job.ExecutePolicy
	} else {
		policyType := rawJob.Lookup("execute_policy", "type")
		if policyType.Type == 0 || policyType.Type == bsontype.Null {
			updates[jobPath+".execute_policy.type"] = job.ExecutePolicy.Type
		}
		if job.ExecutePolicy.MatchRule == "" {
			job.ExecutePolicy.MatchRule = aslanconfig.JobExecutePolicyMatchRuleAll
			updates[jobPath+".execute_policy.match_rule"] = job.ExecutePolicy.MatchRule
		}
		if job.ExecutePolicy.Rules == nil {
			job.ExecutePolicy.Rules = []*commonmodels.JobExecuteRule{}
			updates[jobPath+".execute_policy.rules"] = job.ExecutePolicy.Rules
		}
	}
	if job.ErrorPolicy == nil {
		job.ErrorPolicy = &commonmodels.JobErrorPolicy{
			Policy:        aslanconfig.JobErrorPolicyStop,
			MaximumRetry:  3,
			ApprovalUsers: []*commonmodels.User{},
		}
		updates[jobPath+".error_policy"] = job.ErrorPolicy
	} else {
		if job.ErrorPolicy.Policy == "" {
			job.ErrorPolicy.Policy = aslanconfig.JobErrorPolicyStop
			updates[jobPath+".error_policy.policy"] = job.ErrorPolicy.Policy
		}
		// Inspect BSON so an explicitly configured zero retry count is preserved.
		maximumRetry := rawJob.Lookup("error_policy", "maximum_retry")
		if maximumRetry.Type == 0 || maximumRetry.Type == bsontype.Null {
			job.ErrorPolicy.MaximumRetry = 3
			updates[jobPath+".error_policy.maximum_retry"] = job.ErrorPolicy.MaximumRetry
		}
		if job.ErrorPolicy.ApprovalUsers == nil {
			job.ErrorPolicy.ApprovalUsers = []*commonmodels.User{}
			updates[jobPath+".error_policy.approval_users"] = job.ErrorPolicy.ApprovalUsers
		}
	}
}

func migrateLarkPreferredUsernameClaim510(migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration510LarkPreferredUsernameClaim {
		return nil
	}
	if err := gormtool.Open(config.MysqlUser(), config.MysqlPassword(), config.MysqlHost(), config.MysqlDexDB()); err != nil {
		return fmt.Errorf("failed to open dex database, err: %s", err)
	}

	err := gormtool.DB(config.MysqlDexDB()).Transaction(func(tx *gorm.DB) error {
		var connectors []*connectormodels.Connector
		if err := tx.Where("type IN ?", []string{"lark", "lark_intl"}).Find(&connectors).Error; err != nil {
			return fmt.Errorf("failed to list lark connectors, err: %s", err)
		}
		for _, connector := range connectors {
			config, err := migrateLarkConnectorConfig510(connector.Config)
			if err != nil {
				return fmt.Errorf("failed to migrate lark connector %s config, err: %s", connector.ID, err)
			}
			connector.IncreaseResourceVersion()
			if err := tx.Model(connector).Updates(map[string]interface{}{
				"config":           config,
				"resource_version": connector.ResourceVersion,
			}).Error; err != nil {
				return fmt.Errorf("failed to update lark connector %s, err: %s", connector.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration510LarkPreferredUsernameClaim): true,
	})
}

func migrateLarkConnectorConfig510(config string) (string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(config), &fields); err != nil {
		return "", err
	}
	var emailClaim *string
	if json.Unmarshal(fields["emailClaim"], &emailClaim) != nil || emailClaim == nil {
		return "", fmt.Errorf("emailClaim must be a string")
	}
	fields["preferredUsernameClaim"] = fields["emailClaim"]
	updated, err := json.Marshal(fields)
	return string(updated), err
}

func V510ToV500() error {
	return nil
}

// migrateDockerBuildProxy510 turns on the new per-build proxy switch for existing docker build steps.
// The proxy is injected only when both the build switch and the system proxy are on, so these steps
// keep following the system proxy as they did before 5.1.0.
func migrateDockerBuildProxy510(migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration510DockerBuildProxy {
		return nil
	}

	filter := bson.M{
		"post_build.docker_build":              bson.M{"$ne": nil},
		"post_build.docker_build.enable_proxy": bson.M{"$exists": false},
	}
	update := bson.M{"$set": bson.M{"post_build.docker_build.enable_proxy": true}}
	for _, coll := range []*mongo.Collection{
		commonrepo.NewBuildColl().Collection,
		commonrepo.NewBuildTemplateColl().Collection,
	} {
		res, err := coll.UpdateMany(context.Background(), filter, update)
		if err != nil {
			return fmt.Errorf("failed to set docker build proxy switch in %s, err: %s", coll.Name(), err)
		}
		log.Infof("migration 5.1.0: enabled proxy switch for %d docker build steps in %s", res.ModifiedCount, coll.Name())
	}

	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration510DockerBuildProxy): true,
	})
}
