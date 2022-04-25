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
	return nil
}

func moduleTestingAddNotifyCtls() error {
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
		if testing.NotifyCtl != nil {
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
	_, err = testingCol.BulkWrite(context.TODO(), ms)

	return err
}

func moduleTestingRollBackNotifyCtls() error {
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
	_, err = testingCol.BulkWrite(context.TODO(), ms)

	return err
}

func workflowAddNotifyCtls() error {
	workflowColl := internalmongodb.NewWorkflowColl()

	workflows, err := workflowColl.List(&internalmongodb.ListWorkflowOption{})
	if err != nil {
		return fmt.Errorf("failed to list all data in `zadig.workflow`: %s", err)
	}

	if len(workflows) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, workflow := range workflows {
		if workflow.NotifyCtl != nil {
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
	_, err = workflowColl.BulkWrite(context.TODO(), ms)

	return err
}

func workflowRollBackNotifyCtls() error {
	workflowColl := internalmongodb.NewWorkflowColl()

	workflows, err := workflowColl.List(&internalmongodb.ListWorkflowOption{})
	if err != nil {
		return fmt.Errorf("failed to list all data in `zadig.workflow`: %s", err)
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
	_, err = workflowColl.BulkWrite(context.TODO(), ms)

	return err
}
