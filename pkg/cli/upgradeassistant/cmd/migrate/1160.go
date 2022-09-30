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

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	upgradepath.RegisterHandler("1.15.0", "1.16.0", V1150ToV1160)
	upgradepath.RegisterHandler("1.16.0", "1.15.0", V1150ToV1140)
}

func V1150ToV1160() error {
	if err := addDisplayNameToWorkflowV4(); err != nil {
		log.Errorf("addDisplayNameToWorkflowV4 err:%s", err)
		return err
	}
	return nil
}

func V1160ToV1150() error {

	return nil
}

func addDisplayNameToWorkflowV4() error {
	workflows, _, err := mongodb.NewWorkflowV4Coll().List(&mongodb.ListWorkflowV4Option{}, 0, 0)
	if err != nil {
		return err
	}
	var ms []mongo.WriteModel
	for _, workflow := range workflows {
		if workflow.DisplayName != "" {
			continue
		}
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflow.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"display_name", workflow.Name},
					}},
				}),
		)
	}
	if len(ms) > 0 {
		if _, err := mongodb.NewWorkflowV4Coll().BulkWrite(context.TODO(), ms); err != nil {
			return err
		}
	}

	workflowtasks, _, err := mongodb.NewworkflowTaskv4Coll().List(&mongodb.ListWorkflowTaskV4Option{})
	if err != nil {
		return nil
	}
	var mTasks []mongo.WriteModel
	for _, workflowTask := range workflowtasks {
		if workflowTask.WorkflowDisplayName != "" {
			continue
		}
		mTasks = append(mTasks,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflowTask.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"workflow_display_name", workflowTask.WorkflowName},
					}},
				}),
		)
	}
	if len(mTasks) > 0 {
		if _, err := mongodb.NewworkflowTaskv4Coll().BulkWrite(context.TODO(), ms); err != nil {
			return err
		}
	}

	return nil
}

func addDisplayNameToWorkflow() error {
	workflows, err := mongodb.NewWorkflowColl().List(&mongodb.ListWorkflowOption{})
	if err != nil {
		return err
	}
	var ms []mongo.WriteModel
	for _, workflow := range workflows {
		if workflow.DisplayName != "" {
			continue
		}
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflow.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"display_name", workflow.Name},
					}},
				}),
		)
	}
	if len(ms) > 0 {
		if _, err := mongodb.NewWorkflowV4Coll().BulkWrite(context.TODO(), ms); err != nil {
			return err
		}
	}

	workflowtasks, err := mongodb.NewTaskColl().ListAllTasks(&mongodb.ListAllTaskOption{})
	if err != nil {
		return nil
	}
	var mTasks []mongo.WriteModel
	for _, workflowTask := range workflowtasks {
		if workflowTask.PipelineDisplayName != "" {
			continue
		}
		mTasks = append(mTasks,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflowTask.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"pipeline_display_name", workflowTask.PipelineName},
					}},
				}),
		)
	}
	if len(mTasks) > 0 {
		if _, err := mongodb.NewTaskColl().BulkWrite(context.TODO(), ms); err != nil {
			return err
		}
	}

	return nil
}
