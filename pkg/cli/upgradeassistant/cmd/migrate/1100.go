/*
Copyright 2022 The KodeRover Authors.

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
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	internalmodels "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/orm"
	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"github.com/koderover/zadig/pkg/types"
)

func init() {
	upgradepath.RegisterHandler("1.9.0", "1.10.0", V190ToV1100)
	upgradepath.RegisterHandler("1.10.0", "1.9.0", V1100ToV190)
}

// V190ToV1100 migrates data `caches` and `pre_build.clean_workspace` fields in `zadig.module_build` and `zadig.module_testing`
// to new fields `cache_enable`, `cache_dir_type` and `cache_user_dir`; generate labelBindings for production environment
func V190ToV1100() error {
	log.Info("Migrating data from 1.9.0 to 1.10.0")

	if err := changePolicyCollectionName(); err != nil {
		log.Errorf("Failed to changePolicyCollectionName, err: %s", err)
		return err
	}

	if err := addPresetRoleSystemType(); err != nil {
		log.Errorf("Failed to addPresetRoleSystemType, err: %s", err)
		return err
	}

	if err := addRoleBindingSystemType(); err != nil {
		log.Errorf("Failed to addRoleBindingSystemType, err: %s", err)
		return err
	}

	if err := migrateModuleBuild(); err != nil {
		return fmt.Errorf("failed to migrate data in `zadig.module_build`: %s", err)
	}

	log.Info("Migrate data in `zadig.module_testing`.")
	if err := migrateModuleTesting(); err != nil {
		return fmt.Errorf("failed to migrate data in `zadig.module_testing`: %s", err)
	}

	log.Info("UpdateUserDBTables: ADD cloumn from mysql table `user`.")
	if err := orm.UpdateUserDBTables(orm.DbEditActionAdd); err != nil {
		return fmt.Errorf("UpdateUserDBTables: failed to ADD cloumn from mysql table `user`: %s", err)
	}
	return nil
}

func changeToCustomType() error {
	ctx := context.Background()

	var res []*models.RoleBinding
	cursor, err := newRoleBindingColl().Find(ctx, bson.M{})
	if err != nil {
		log.Errorf("Failed to Find Policies, err: %s", err)
		return err
	}

	err = cursor.All(ctx, &res)
	if err != nil {
		return err
	}
	for _, v := range res {
		if v.Namespace == "*" && v.RoleRef.Name == "admin" && v.Type != "" {
			continue
		}
		query := bson.M{"name": v.Name}
		change := bson.M{"$set": bson.M{
			"type": setting.ResourceTypeCustom,
		}}
		_, err := newRoleBindingColl().UpdateOne(ctx, query, change)
		if err != nil {
			return err
		}

	}
	return nil
}

func changeToSystemType() error {
	query := bson.M{"namespace": "*", "role_ref.name": "admin"}
	change := bson.M{"$set": bson.M{
		"type": setting.ResourceTypeSystem,
	}}
	_, err := newRoleBindingColl().UpdateOne(context.TODO(), query, change)
	if err != nil {
		return err
	}
	return nil
}

func addRoleBindingSystemType() error {
	if err := changeToSystemType(); err != nil {
		return err
	}
	return changeToCustomType()
}

func addPresetRoleSystemType() error {
	ctx := context.Background()

	var res []*models.Role
	cursor, err := newRoleColl().Find(ctx, bson.M{})
	if err != nil {
		log.Errorf("Failed to Find Policies, err: %s", err)
		return err
	}

	err = cursor.All(ctx, &res)
	if err != nil {
		return err
	}
	for _, v := range res {
		if v.Namespace == "*" || v.Namespace == "" {
			query := bson.M{"name": v.Name}
			change := bson.M{"$set": bson.M{
				"type": setting.ResourceTypeSystem,
			}}
			_, err := newRoleColl().UpdateOne(ctx, query, change)
			if err != nil {
				return err
			}
		} else {
			query := bson.M{"name": v.Name}
			change := bson.M{"$set": bson.M{
				"type": setting.ResourceTypeCustom,
			}}
			_, err := newRoleColl().UpdateOne(ctx, query, change)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func changePolicyCollectionName() error {
	var res []*models.PolicyMeta

	ctx := context.Background()
	cursor, err := newPolicyColl().Find(ctx, bson.M{})
	if err != nil {
		log.Errorf("Failed to Find Policies, err: %s", err)
		return err
	}

	err = cursor.All(ctx, &res)
	if err != nil {
		return err
	}

	for _, v := range res {
		query := bson.M{"resource": v.Resource}
		opts := options.Replace().SetUpsert(true)
		_, err := newPolicyMetaColl().ReplaceOne(context.TODO(), query, v, opts)
		if err != nil {
			log.Errorf("relace one err:%v ,resource:%v", err, v.Resource)
		}
	}
	//delete collection
	return newPolicyColl().Drop(ctx)
}

func rollbackChangePolicyCollectionName() error {
	var res []*models.PolicyMeta
	ctx := context.Background()
	cursor, err := newPolicyMetaColl().Find(ctx, bson.M{})
	if err != nil {
		return err
	}

	err = cursor.All(ctx, &res)
	if err != nil {
		return err
	}

	var ois []interface{}
	for _, obj := range res {
		ois = append(ois, obj)
	}
	if _, err = newPolicyColl().InsertMany(ctx, ois); err != nil {
		log.Errorf("Failed to InsertMany, err: %s", err)
		return err
	}
	//delete collection
	return newPolicyMetaColl().Drop(ctx)
}

func V1100ToV190() error {
	log.Info("Rollback data from 1.10.0 to 1.9.0")
	if err := rollbackChangePolicyCollectionName(); err != nil {
		log.Errorf("Failed to rollbackChangePolicyCollectionName,err: %s", err)
		return err
	}

	log.Info("UpdateUserDBTables: drop cloumn from mysql table `user`.")
	if err := orm.UpdateUserDBTables(orm.DbEditActionDrop); err != nil {
		return fmt.Errorf("UpdateUserDBTables: failed to drop cloumn from mysql table `user`: %s", err)
	}
	return nil
}

func newPolicyColl() *mongo.Collection {
	collection := mongotool.Database(fmt.Sprintf("%s_policy", config.MongoDatabase())).Collection("policy")
	return collection
}

func newRoleColl() *mongo.Collection {
	collection := mongotool.Database(fmt.Sprintf("%s_policy", config.MongoDatabase())).Collection("role")
	return collection
}

func newPolicyMetaColl() *mongo.Collection {
	collection := mongotool.Database(fmt.Sprintf("%s_policy", config.MongoDatabase())).Collection("policy_meta")
	return collection
}

func migrateModuleBuild() error {
	buildCol := internalmongodb.NewBuildColl()

	builds, err := buildCol.List(&internalmongodb.BuildListOption{})
	if err != nil {
		return fmt.Errorf("failed to list all data in `zadig.module_build`: %s", err)
	}

	if len(builds) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, build := range builds {
		if err := migrateOneBuild(build); err != nil {
			log.Errorf("Failed to migrate data: %v. Err: %s", build, err)
			continue
		}

		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", build.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"cache_enable", build.CacheEnable},
						{"cache_dir_type", build.CacheDirType},
						{"cache_user_dir", build.CacheUserDir},
						{"advanced_setting_modified", build.AdvancedSettingsModified},
					}},
				}),
		)
	}

	_, err = buildCol.BulkWrite(context.TODO(), ms)

	return err
}

func migrateModuleTesting() error {
	testingCol := internalmongodb.NewTestingColl()

	testings, err := testingCol.List(&internalmongodb.ListTestOption{})
	if err != nil {
		return fmt.Errorf("failed to list all data in `zadig.module_testing`: %s", err)
	}

	if len(testings) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, testing := range testings {
		if err := migrateOneTesting(testing); err != nil {
			log.Errorf("Failed to migrate data: %v. Err: %s", testing, err)
			continue
		}

		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", testing.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"cache_enable", testing.CacheEnable},
						{"cache_dir_type", testing.CacheDirType},
						{"cache_user_dir", testing.CacheUserDir},
						{"advanced_setting_modified", testing.AdvancedSettingsModified},
					}},
				}),
		)
	}

	_, err = testingCol.BulkWrite(context.TODO(), ms)

	return err
}

func migrateOneBuild(build *internalmodels.Build) error {
	if build.PreBuild != nil && build.PreBuild.CleanWorkspace {
		return nil
	}

	build.CacheEnable = true

	if len(build.Caches) == 0 {
		build.CacheDirType = types.WorkspaceCacheDir
		return nil
	}

	build.CacheDirType = types.UserDefinedCacheDir
	build.CacheUserDir = build.Caches[0]

	build.AdvancedSettingsModified = buildHasModifiedAdvancedSetting(build)

	return nil
}

func migrateOneTesting(testing *internalmodels.Testing) error {
	if testing.PreTest != nil && testing.PreTest.CleanWorkspace {
		return nil
	}

	testing.CacheEnable = true

	if len(testing.Caches) == 0 {
		testing.CacheDirType = types.WorkspaceCacheDir
		return nil
	}

	testing.CacheDirType = types.UserDefinedCacheDir
	testing.CacheUserDir = testing.Caches[0]

	testing.AdvancedSettingsModified = testHasModifiedAdvancedSetting(testing)

	return nil
}

func buildHasModifiedAdvancedSetting(build *internalmodels.Build) bool {
	// default timeout is 60
	if build.Timeout != 60 {
		return true
	}

	// cache is disabled by default
	if build.CacheEnable {
		return true
	}

	// default is local cluster, but it is a relatively new field, so we also check for empty
	if build.PreBuild.ClusterID != "" && build.PreBuild.ClusterID != setting.LocalClusterID {
		return true
	}

	if build.PreBuild.ResReq != setting.LowRequest {
		return true
	}

	return false
}

func testHasModifiedAdvancedSetting(testing *internalmodels.Testing) bool {
	// default no artifact path exists
	if testing.ArtifactPaths != nil && len(testing.ArtifactPaths) > 0 {
		return true
	}

	// default timeout is 60
	if testing.Timeout != 60 {
		return true
	}

	// cache is disabled by default
	if testing.CacheEnable {
		return true
	}

	// default is local cluster, but it is a relatively new field, so we also check for empty
	if testing.PreTest.ClusterID != "" && testing.PreTest.ClusterID != setting.LocalClusterID {
		return true
	}

	if testing.PreTest.ResReq != setting.LowRequest {
		return true
	}

	if testing.HookCtl != nil && testing.HookCtl.Enabled {
		return true
	}

	if testing.NotifyCtl != nil && testing.NotifyCtl.Enabled {
		return true
	}

	if testing.Schedules != nil && testing.Schedules.Enabled {
		return true
	}

	return false
}
