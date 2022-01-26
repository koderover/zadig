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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	labelModel "github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
	labelMongodb "github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"

	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

func init() {
	upgradepath.RegisterHandler("1.9.0", "1.10.0", V190ToV1100)
	upgradepath.RegisterHandler("1.10.0", "1.9.0", V1100ToV190)
}

// V190ToV1100 generate labelBindings for production environment
func V190ToV1100() error {
	log.Info("Migrating data from 1.9.0 to 1.10.0")
	if err := generateProductionEnv(); err != nil {
		log.Errorf("Failed to generateProductionEnv, err: %s", err)
		return err
	}
	if err := changePolicyCollectionName(); err != nil {
		log.Errorf("Failed to changePolicyCollectionName, err: %s", err)
		return err
	}
	return nil
}

func changePolicyCollectionName() error {
	var res []*models.PolicyMeta
	//
	ctx := context.Background()
	cursor, err := newPolicyColl().Find(ctx, bson.M{})
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
	if _, err = newPolicyMetaColl().InsertMany(context.TODO(), ois); err != nil {
		return err
	}
	//delete collection
	return newPolicyColl().Drop(ctx)
}

func V1100ToV190() error {
	if err := deleteLabelAndLabelBindings(); err != nil {
		log.Errorf("Failed to generateProductionEnv, err: %s", err)
		return err
	}
	return nil
}

func deleteLabelAndLabelBindings() error {
	lisOpt := labelMongodb.ListLabelOpt{
		[]labelMongodb.Label{
			{
				Key:   "production",
				Value: "false",
			},
			{
				Key:   "production",
				Value: "true",
			},
		},
	}
	labels, err := newLabelColl().List(lisOpt)
	if err != nil {
		return err
	}
	if len(labels) != 2 {
		log.Errorf("production labels len not equal 2,current is %d\n", len(labels))
		return fmt.Errorf("production labels len not equal 2,current is %d\n", len(labels))
	}
	var labelIDs []string
	for _, label := range labels {
		labelIDs = append(labelIDs, label.ID.Hex())
	}

	res, err := mongodb.NewLabelBindingColl().ListByOpt(&mongodb.LabelBindingCollFindOpt{LabelIDs: labelIDs})
	if err != nil {
		log.Errorf("list labelbingding err:%s", err)
		return err
	}

	var labelBindingIDs []string
	for _, labelBinding := range res {
		labelBindingIDs = append(labelBindingIDs, labelBinding.ID.Hex())
	}

	if err := mongodb.NewLabelBindingColl().BulkDeleteByIds(labelBindingIDs); err != nil {
		return err
	}
	return mongodb.NewLabelColl().BulkDelete(labelIDs)

}

func generateProductionEnv() error {
	// create label key=product value=true
	err := createLabels()
	if err != nil {
		log.Errorf("fail to createLabels , err:%s", err)
		return err
	}

	// create labelBindings for environment resources
	if err := createLabelBindings(); err != nil {
		log.Errorf("fail to createLabelBindings,err:%s", err)
		return err
	}

	return nil
}

func newLabelColl() *labelMongodb.LabelColl {
	name := labelModel.Label{}.TableName()
	return &labelMongodb.LabelColl{
		Collection: mongotool.Database(fmt.Sprintf("%s", config.MongoDatabase())).Collection(name),
	}
}

func newPolicyColl() *mongo.Collection {
	collection := mongotool.Database(fmt.Sprintf("%s_policy", config.MongoDatabase())).Collection("policy")
	return collection
}

func newPolicyMetaColl() *mongo.Collection {
	collection := mongotool.Database(fmt.Sprintf("%s_policy", config.MongoDatabase())).Collection("policy_meta")
	return collection
}

func createLabels() error {
	var toCreateLabels []*labelModel.Label
	productEnvLabel := &labelModel.Label{
		Type:       "system",
		Key:        "production",
		Value:      "true",
		CreateBy:   "system",
		CreateTime: time.Now().Unix(),
	}
	nonProductEnvLabel := &labelModel.Label{
		Type:       "system",
		Key:        "production",
		Value:      "false",
		CreateBy:   "system",
		CreateTime: time.Now().Unix(),
	}
	toCreateLabels = append(toCreateLabels, productEnvLabel, nonProductEnvLabel)
	if err := newLabelColl().BulkCreate(toCreateLabels); err != nil {
		return err
	}
	return nil
}

func createLabelBindings() error {
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{})
	if err != nil {
		return err
	}
	clusterMap := make(map[string]*commonmodels.K8SCluster)
	clusters, err := commonrepo.NewK8SClusterColl().List(nil)
	if err != nil {
		log.Errorf("Failed to list clusters in db, err: %s", err)
		return err
	}

	for _, cls := range clusters {
		clusterMap[cls.ID.Hex()] = cls
	}
	lisOpt := labelMongodb.ListLabelOpt{
		[]labelMongodb.Label{
			{
				Key:   "production",
				Value: "false",
			},
			{
				Key:   "production",
				Value: "true",
			},
		},
	}
	labels, err := newLabelColl().List(lisOpt)
	if err != nil {
		return err
	}
	if len(labels) != 2 {
		return fmt.Errorf("production labels len not equal 2")
	}
	var productLabelID, nonProductLabelID string
	for _, label := range labels {
		if label.Value == "false" {
			nonProductLabelID = label.ID.Hex()
		} else {
			productLabelID = label.ID.Hex()
		}
	}
	var labelBindings []*labelMongodb.LabelBinding
	for _, env := range envs {
		lb := &labelMongodb.LabelBinding{
			Resource: labelMongodb.Resource{
				Name:        env.EnvName,
				ProjectName: env.ProductName,
				Type:        "Environment",
			},
			CreateBy: "system",
		}
		cluster, ok := clusterMap[env.ClusterID]
		if ok && cluster.Production {
			lb.LabelID = productLabelID
		} else {
			lb.LabelID = nonProductLabelID
		}
		labelBindings = append(labelBindings, lb)
	}

	if err := labelMongodb.NewLabelBindingColl().CreateMany(labelBindings); err != nil {
		return err
	}
	return nil
}
