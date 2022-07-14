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

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	upgradepath.RegisterHandler("1.11.0", "1.12.0", V1110ToV1120)
	upgradepath.RegisterHandler("1.12.0", "1.11.0", V1120ToV1110)
}

func V1110ToV1120() error {
	// iterate all testings , if contains notifyctl , add notifyctls
	if err := moduleTestingAddNotifyCtls(); err != nil {
		log.Errorf("moduleTestingAddNotifyCtls err:%s", err)
		return err
	}
	// iterate all workflows , if contains notifyctl , add notifyctls
	if err := workflowAddNotifyCtls(); err != nil {
		log.Errorf("workflowAddNotifyCtls err:%s", err)
		return err
	}

	if err := buildAddJenkinsID(); err != nil {
		log.Errorf("buildAddJenkinsID err:%s", err)
		return err
	}

	if err := integrateSonar(); err != nil {
		log.Errorf("add sonar integration err:%s", err)
		return err
	}

	if err := supplementCodehostAlias(); err != nil {
		log.Errorf("supplement codehost alias, err:%s", err)
		return err
	}

	return nil
}

func V1120ToV1110() error {
	// iterate all testings , if contains notifyctl , add notifyctls
	if err := moduleTestingRollBackNotifyCtls(); err != nil {
		log.Errorf("moduleTestingRollBackNotifyCtls err:%s", err)
		return err
	}
	// iterate all workflows , if contains notifyctl , add notifyctls
	if err := workflowRollBackNotifyCtls(); err != nil {
		log.Errorf("workflowRollBackNotifyCtls err:%s", err)
		return err
	}

	if err := buildAddJenkinsIDRollBack(); err != nil {
		log.Errorf("buildAddJenkinsIDRollBack err:%s", err)
		return err
	}

	if err := supplementCodehostAliasRollBack(); err != nil {
		log.Errorf("supplement codehost alias rollback, err:%s", err)
		return err
	}
	return nil
}

func moduleTestingAddNotifyCtls() error {
	testingCol := internalmongodb.NewTestingColl()

	testings, err := testingCol.List(&internalmongodb.ListTestOption{})
	if err != nil {
		return fmt.Errorf("failed to list all data in `zadig.module_testing`,err: %s", err)
	}

	if len(testings) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, testing := range testings {
		// 如果已经执行过数据迁移，不要重复迁移
		if testing.NotifyCtls != nil {
			continue
		}
		if testing.NotifyCtl != nil && testing.NotifyCtl.Enabled {
			testing.NotifyCtls = []*models.NotifyCtl{testing.NotifyCtl}
		}

		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", testing.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"notify_ctls", testing.NotifyCtls},
					}},
				}),
		)
	}
	if len(ms) > 0 {
		_, err = testingCol.BulkWrite(context.TODO(), ms)
	}

	return err
}

func moduleTestingRollBackNotifyCtls() error {
	testingCol := internalmongodb.NewTestingColl()

	testings, err := testingCol.List(&internalmongodb.ListTestOption{})
	if err != nil {
		return fmt.Errorf("failed to list all data in `zadig.module_testing`,err: %s", err)
	}

	if len(testings) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, testing := range testings {
		for _, notifyctl := range testing.NotifyCtls {
			if notifyctl != nil && notifyctl.Enabled {
				testing.NotifyCtl = notifyctl
				break
			}
		}

		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", testing.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"notify_ctl", testing.NotifyCtl},
					}},
				}),
		)
	}
	if len(ms) > 0 {
		_, err = testingCol.BulkWrite(context.TODO(), ms)
	}

	return err
}

func workflowAddNotifyCtls() error {
	workflowColl := internalmongodb.NewWorkflowColl()

	workflows, err := workflowColl.List(&internalmongodb.ListWorkflowOption{})
	if err != nil {
		return fmt.Errorf("failed to list all data in `zadig.workflow`,err: %s", err)
	}

	if len(workflows) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, workflow := range workflows {
		// 如果已经执行过数据迁移，不要重复迁移
		if workflow.NotifyCtls != nil {
			continue
		}
		if workflow.NotifyCtl != nil && workflow.NotifyCtl.Enabled {
			workflow.NotifyCtls = []*models.NotifyCtl{workflow.NotifyCtl}
		}

		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflow.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"notify_ctls", workflow.NotifyCtls},
					}},
				}),
		)
	}
	if len(ms) > 0 {
		_, err = workflowColl.BulkWrite(context.TODO(), ms)
	}

	return err
}

func workflowRollBackNotifyCtls() error {
	workflowColl := internalmongodb.NewWorkflowColl()

	workflows, err := workflowColl.List(&internalmongodb.ListWorkflowOption{})
	if err != nil {
		return fmt.Errorf("failed to list all data in `zadig.workflow`,err: %s", err)
	}

	if len(workflows) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, workflow := range workflows {
		for _, notifyctl := range workflow.NotifyCtls {
			if notifyctl != nil && notifyctl.Enabled {
				workflow.NotifyCtl = notifyctl
				break
			}
		}

		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflow.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"notify_ctl", workflow.NotifyCtl},
					}},
				}),
		)
	}
	if len(ms) > 0 {
		_, err = workflowColl.BulkWrite(context.TODO(), ms)
	}

	return err
}

// If the data is not cleaned, executing the jenkins build will report an error
func buildAddJenkinsID() error {
	jenkins, err := internalmongodb.NewJenkinsIntegrationColl().List()
	if err != nil {
		return fmt.Errorf("failed to list jenkins integration ,err: %s", err)
	}

	if len(jenkins) == 0 {
		return nil
	}

	builds, err := internalmongodb.NewBuildColl().List(&internalmongodb.BuildListOption{})
	if err != nil {
		return fmt.Errorf("failed to list all builds ,err: %s", err)
	}

	if len(builds) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, build := range builds {
		if build.JenkinsBuild == nil {
			continue
		}

		if build.JenkinsBuild.JenkinsID != "" {
			continue
		}

		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", build.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"jenkins_build.jenkins_id", jenkins[0].ID.Hex()},
					}},
				}),
		)
	}
	if len(ms) > 0 {
		_, err = internalmongodb.NewBuildColl().BulkWrite(context.TODO(), ms)
	}

	return err
}

func integrateSonar() error {
	sonarImageList, err := internalmongodb.NewBasicImageColl().GetSonarTypeImage()
	if err != nil {
		return fmt.Errorf("failed to list basic images of type sonar ,err: %s", err)
	}

	// if no sonar image list is included, we add one
	if len(sonarImageList) == 0 {
		return internalmongodb.NewBasicImageColl().CreateZadigSonarImage()
	}
	return nil
}

func buildAddJenkinsIDRollBack() error {
	builds, err := internalmongodb.NewBuildColl().List(&internalmongodb.BuildListOption{})
	if err != nil {
		return fmt.Errorf("failed to list all builds ,err: %s", err)
	}

	if len(builds) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, build := range builds {
		if build.JenkinsBuild == nil {
			continue
		}

		if build.JenkinsBuild.JenkinsID == "" {
			continue
		}

		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", build.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"jenkins_build.jenkins_id", ""},
					}},
				}),
		)
	}
	if len(ms) > 0 {
		_, err = internalmongodb.NewBuildColl().BulkWrite(context.TODO(), ms)
	}
	return err

}
func supplementCodehostAlias() error {
	codeHosts, err := internalmongodb.NewCodehostColl().List()
	if err != nil {
		return fmt.Errorf("failed to list all codehosts ,err: %s", err)
	}
	var ms []mongo.WriteModel
	for _, codehost := range codeHosts {
		if codehost.Alias != "" {
			continue
		}
		var alias string
		switch codehost.Type {
		case "gitlab":
			alias = codehost.AccessKey[0:8]
		case "github":
			alias = codehost.Namespace
		case "gerrit":
			alias = codehost.Username
		case "gitee":
			alias = codehost.Namespace
		}
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"id", codehost.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"alias", alias},
					}},
				}),
		)
	}

	if len(ms) > 0 {
		_, err = internalmongodb.NewCodehostColl().BulkWrite(context.TODO(), ms)
	}
	return err
}

func supplementCodehostAliasRollBack() error {
	codeHosts, err := internalmongodb.NewCodehostColl().List()
	if err != nil {
		return fmt.Errorf("failed to list all codehosts ,err: %s", err)
	}
	var ms []mongo.WriteModel
	for _, codehost := range codeHosts {
		if codehost.Alias == "" {
			continue
		}
		if codehost.Type == "other" {
			continue
		}

		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"id", codehost.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"alias", ""},
					}},
				}),
		)
	}

	if len(ms) > 0 {
		_, err = internalmongodb.NewCodehostColl().BulkWrite(context.TODO(), ms)
	}
	return err
}
