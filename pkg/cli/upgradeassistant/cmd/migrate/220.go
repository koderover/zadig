/*
Copyright 2023 The KodeRover Authors.

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

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	upgradepath.RegisterHandler("2.1.0", "2.2.0", V210ToV220)
	upgradepath.RegisterHandler("2.2.0", "2.1.0", V220ToV210)
}

func V210ToV220() error {
	log.Infof("-------- start migrate predeploy to build --------")
	err := migratePreDeployToBuild()
	if err != nil {
		log.Errorf("migratePreDeployToBuild error: %s", err)
		return err
	}

	return nil
}

func V220ToV210() error {
	return nil
}

func migratePreDeployToBuild() error {
	cursor, err := mongodb.NewBuildColl().ListByCursor(&mongodb.BuildListOption{})
	if err != nil {
		return err
	}
	var ms []mongo.WriteModel
	for cursor.Next(context.Background()) {
		var build models.Build
		if err := cursor.Decode(&build); err != nil {
			return err
		}

		if build.PreDeploy == nil {
			build.PreDeploy = &models.PreDeploy{}
			build.PreDeploy.BuildOS = build.PreBuild.BuildOS
			build.PreDeploy.ImageFrom = build.PreBuild.ImageFrom
			build.PreDeploy.ImageID = build.PreBuild.ImageID
			build.PreDeploy.Installs = build.PreBuild.Installs

			build.DeployInfrastructure = build.Infrastructure
			build.DeployVMLabels = build.VMLabels
			build.DeployRepos = build.Repos

			ms = append(ms,
				mongo.NewUpdateOneModel().
					SetFilter(bson.D{{"_id", build.ID}}).
					SetUpdate(bson.D{{"$set",
						bson.D{
							{"pre_deploy", build.PreDeploy},
							{"deploy_infrastructure", build.DeployInfrastructure},
							{"deploy_vm_labels", build.DeployVMLabels},
							{"deploy_repos", build.DeployRepos},
						}},
					}),
			)
			log.Infof("add build %s", build.Name)
		}

		if len(ms) >= 50 {
			log.Infof("update %d build", len(ms))
			if _, err := mongodb.NewBuildColl().BulkWrite(context.TODO(), ms); err != nil {
				return fmt.Errorf("update builds error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		log.Infof("update %d build", len(ms))
		if _, err := mongodb.NewBuildColl().BulkWrite(context.TODO(), ms); err != nil {
			return fmt.Errorf("udpate builds error: %s", err)
		}
	}

	return nil
}
