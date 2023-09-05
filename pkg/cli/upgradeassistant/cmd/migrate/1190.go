/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package migrate

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	upgradepath.RegisterHandler("1.18.0", "1.19.0", V1180ToV1190)
	upgradepath.RegisterHandler("1.19.0", "1.18.0", V1190ToV1180)
}

func V1180ToV1190() error {
	log.Infof("-------- start migrate cluster workflow schedule strategy --------")
	if err := migrateClusterScheduleStrategy(); err != nil {
		log.Errorf("migrateClusterScheduleStrategy err: %v", err)
		return err
	}

	log.Infof("-------- start migrate workflow template --------")
	if err := migrateWorkflowTemplate(); err != nil {
		log.Infof("migrateWorkflowTemplate err: %v", err)
		return err
	}

	return nil
}

func V1190ToV1180() error {
	return nil
}

func migrateClusterScheduleStrategy() error {
	coll := mongodb.NewK8SClusterColl()
	clusters, err := coll.List(nil)
	if err != nil {
		return fmt.Errorf("failed to get all cluster from db, err: %v", err)
	}

	for _, cluster := range clusters {
		if cluster.AdvancedConfig != nil && cluster.AdvancedConfig.ScheduleStrategy != nil {
			continue
		}

		if cluster.AdvancedConfig == nil {
			cluster.AdvancedConfig = &models.AdvancedConfig{
				ClusterAccessYaml: kube.ClusterAccessYamlTemplate,
				ScheduleWorkflow:  true,
				ScheduleStrategy: []*models.ScheduleStrategy{
					{
						StrategyID:   primitive.NewObjectID().Hex(),
						StrategyName: setting.NormalScheduleName,
						Strategy:     setting.NormalSchedule,
						Default:      true,
					},
				},
			}
		} else {
			cluster.AdvancedConfig.ScheduleStrategy = make([]*models.ScheduleStrategy, 0)
			strategy := &models.ScheduleStrategy{
				StrategyID:  primitive.NewObjectID().Hex(),
				Strategy:    cluster.AdvancedConfig.Strategy,
				NodeLabels:  cluster.AdvancedConfig.NodeLabels,
				Tolerations: cluster.AdvancedConfig.Tolerations,
				Default:     true,
			}
			switch strategy.Strategy {
			case setting.NormalSchedule:
				strategy.StrategyName = setting.NormalScheduleName
			case setting.RequiredSchedule:
				strategy.StrategyName = setting.RequiredScheduleName
			case setting.PreferredSchedule:
				strategy.StrategyName = setting.PreferredScheduleName
			}
			cluster.AdvancedConfig.ScheduleStrategy = append(cluster.AdvancedConfig.ScheduleStrategy, strategy)

		}
		err := coll.UpdateScheduleStrategy(cluster)
		if err != nil {
			return fmt.Errorf("failed to update cluster in ua method migrateClusterScheduleStrategy, err: %v", err)
		}
	}
	return nil
}

var oldWorkflowTemplates = []string{
	"业务变更及测试", "数据库及业务变更", "多环境服务变更", "多阶段灰度", "istio发布", "Nacos 配置变更及服务升级", "Apollo 配置变更及服务升级",
}

func migrateWorkflowTemplate() error {
	// delete old workflow templates
	for _, name := range oldWorkflowTemplates {
		query := bson.M{
			"template_name": name,
			"created_by":    setting.SystemUser,
		}
		_, err := mongodb.NewWorkflowV4TemplateColl().DeleteOne(context.TODO(), query)
		if err != nil {
			return fmt.Errorf("failed to delete old workflow template %s for merging custom and release workflow, err: %v", name, err)
		}
	}

	// change release workflow type to common_workflow for merge release and custom workflow
	cursor, err := mongodb.NewWorkflowV4Coll().ListByCursor(&mongodb.ListWorkflowV4Option{Category: setting.ReleaseWorkflow})
	if err != nil {
		return fmt.Errorf("failed to list workflowV4 for merging custom and release workflow, err: %v", err)
	}
	var ms []mongo.WriteModel
	for cursor.Next(context.Background()) {
		var workflow models.WorkflowV4
		if err := cursor.Decode(&workflow); err != nil {
			return err
		}
		if workflow.Category == setting.ReleaseWorkflow {
			ms = append(ms,
				mongo.NewUpdateOneModel().
					SetFilter(bson.D{{"_id", workflow.ID}}).
					SetUpdate(bson.D{{"$set",
						bson.D{
							{"category", setting.CustomWorkflow},
						}},
					}),
			)
		}
		if len(ms) >= 50 {
			log.Infof("update %d workflowV4", len(ms))
			if _, err := mongodb.NewWorkflowV4Coll().BulkWrite(context.TODO(), ms); err != nil {
				return fmt.Errorf("udpate workflowV4s for merging custom and release workflow, error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		log.Infof("update %d workflowV4s", len(ms))
		if _, err := mongodb.NewWorkflowV4Coll().BulkWrite(context.TODO(), ms); err != nil {
			return fmt.Errorf("udpate workflowV4s for merging custom and release workflow, error: %s", err)
		}
	}
	return nil
}
