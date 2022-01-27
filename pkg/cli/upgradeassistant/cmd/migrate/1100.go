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

	internalmodels "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

func init() {
	upgradepath.RegisterHandler("1.9.0", "1.10.0", V190ToV1100)
	upgradepath.RegisterHandler("1.10.0", "1.9.0", V1100ToV190)
}

// V190ToV1100 migrates data `caches` and `pre_build.clean_workspace` fields in `zadig.module_build` and `zadig.module_testing`
// to new fields `cache_enable`, `cache_dir_type` and `cache_user_dir`.
func V190ToV1100() error {
	log.Info("Migrate data from 1.9.0 to 1.10.0.")

	log.Info("Migrate data in `zadig.module_build`.")
	if err := migrateModuleBuild(); err != nil {
		return fmt.Errorf("failed to migrate data in `zadig.module_build`: %s", err)
	}

	log.Info("Migrate data in `zadig.module_testing`.")
	if err := migrateModuleTesting(); err != nil {
		return fmt.Errorf("failed to migrate data in `zadig.module_testing`: %s", err)
	}

	return nil
}

// Since the old data has not been changed, no changes are required.
func V1100ToV190() error {
	log.Info("Rollback data from 1.10.0 to 1.9.0")
	return nil
}

func migrateModuleBuild() error {
	buildCol := internalmongodb.NewBuildColl()

	builds, err := buildCol.List(&internalmongodb.BuildListOption{})
	if err != nil {
		return fmt.Errorf("failed to list all data in `zadig.module_build`: %s", err)
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
						{"cache_user_dir", build.CacheUserDir}}},
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
						{"cache_user_dir", testing.CacheUserDir}}},
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

	return nil
}
