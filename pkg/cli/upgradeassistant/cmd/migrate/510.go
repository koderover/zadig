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
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	internalmodels "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func init() {
	upgradepath.RegisterHandler("5.0.0", "5.1.0", V500ToV510)
	upgradepath.RegisterHandler("5.1.0", "5.0.0", V510ToV500)
}

// V500ToV510 executes 5.1.0 upgrade steps.
func V500ToV510() error {
	migrationInfo, err := getMigrationInfo()
	if err != nil {
		return fmt.Errorf("failed to get migration info from db, err: %s", err)
	}

	defer func() {
		updateMigrationError(migrationInfo.ID, err)
	}()

	err = migrateDockerBuildProxy510(migrationInfo)
	if err != nil {
		return err
	}

	return nil
}

func V510ToV500() error {
	return nil
}

// migrateDockerBuildProxy510 sets the new per-build proxy switch for existing docker build steps.
// Before 5.1.0 these steps used the system proxy whenever it was enabled with an HTTP/HTTPS type,
// so the switch is initialized from the system proxy to keep their previous behavior.
func migrateDockerBuildProxy510(migrationInfo *internalmodels.Migration) error {
	if migrationInfo.Migration510DockerBuildProxy {
		return nil
	}

	proxies, err := commonrepo.NewProxyColl().List(&commonrepo.ProxyArgs{})
	if err != nil {
		return fmt.Errorf("failed to list proxies, err: %s", err)
	}
	enableProxy := len(proxies) > 0 && proxies[0].EnableRepoProxy &&
		(proxies[0].Type == "http" || proxies[0].Type == "https")

	filter := bson.M{
		"post_build.docker_build":              bson.M{"$ne": nil},
		"post_build.docker_build.enable_proxy": bson.M{"$exists": false},
	}
	update := bson.M{"$set": bson.M{"post_build.docker_build.enable_proxy": enableProxy}}
	for _, coll := range []*mongo.Collection{
		commonrepo.NewBuildColl().Collection,
		commonrepo.NewBuildTemplateColl().Collection,
	} {
		res, err := coll.UpdateMany(context.Background(), filter, update)
		if err != nil {
			return fmt.Errorf("failed to set docker build proxy switch in %s, err: %s", coll.Name(), err)
		}
		log.Infof("migration 5.1.0: set enable_proxy=%t for %d docker build steps in %s", enableProxy, res.ModifiedCount, coll.Name())
	}

	return internalmongodb.NewMigrationColl().UpdateMigrationStatus(migrationInfo.ID, map[string]interface{}{
		getMigrationFieldBsonTag(migrationInfo, &migrationInfo.Migration510DockerBuildProxy): true,
	})
}
