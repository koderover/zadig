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

package mongodb

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type CustomWorkflowTestReportColl struct {
	*mongo.Collection

	coll string
}

func NewCustomWorkflowTestReportColl() *CustomWorkflowTestReportColl {
	name := models.CustomWorkflowTestReport{}.TableName()
	return &CustomWorkflowTestReportColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *CustomWorkflowTestReportColl) GetCollectionName() string {
	return c.coll
}

func (c *CustomWorkflowTestReportColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "workflow_name", Value: 1},
				bson.E{Key: "job_name", Value: 1},
				bson.E{Key: "task_id", Value: 1},
				bson.E{Key: "zadig_test_name", Value: 1},
				bson.E{Key: "service_name", Value: 1},
				bson.E{Key: "service_module", Value: 1},
			},
			Options: options.Index().SetUnique(false).SetName("report_index"),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *CustomWorkflowTestReportColl) Create(args *models.CustomWorkflowTestReport) error {
	if args == nil {
		return errors.New("nil custom workflow test report")
	}

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *CustomWorkflowTestReportColl) ListByWorkflowJobName(workflowName, jobName string, taskID int64) ([]*models.CustomWorkflowTestReport, error) {
	resp := make([]*models.CustomWorkflowTestReport, 0)
	jobName = strings.ToLower(jobName)

	query := bson.M{
		"workflow_name": workflowName,
		"job_name":      jobName,
		"task_id":       taskID,
	}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)
	return resp, err
}

func (c *CustomWorkflowTestReportColl) ListByWorkflowJobTaskName(workflowName, jobTaskName string, taskID int64) ([]*models.CustomWorkflowTestReport, error) {
	jobTaskName = strings.ToLower(jobTaskName)

	// 先获取一条按 retry_num 降序排序的记录，找到最大值
	query := bson.M{
		"workflow_name": workflowName,
		"job_task_name": jobTaskName,
		"task_id":       taskID,
	}

	opts := options.FindOne().SetSort(bson.D{bson.E{Key: "retry_num", Value: -1}})

	var maxRecord models.CustomWorkflowTestReport
	err := c.Collection.FindOne(context.TODO(), query, opts).Decode(&maxRecord)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return []*models.CustomWorkflowTestReport{}, nil
		}
		return nil, err
	}

	// 查询所有具有最大 retry_num 的记录
	query["retry_num"] = maxRecord.RetryNum
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	var result []*models.CustomWorkflowTestReport
	err = cursor.All(context.TODO(), &result)
	return result, err
}
