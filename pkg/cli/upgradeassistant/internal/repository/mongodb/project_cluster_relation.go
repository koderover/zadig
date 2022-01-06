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

package mongodb

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/mongo"

	models "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type ProjectClusterRelationColl struct {
	*mongo.Collection

	coll string
}

func NewProjectClusterRelationColl() *ProjectClusterRelationColl {
	name := models.ProjectClusterRelation{}.TableName()
	return &ProjectClusterRelationColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ProjectClusterRelationColl) GetCollectionName() string {
	return c.coll
}

func (c *ProjectClusterRelationColl) Create(args *models.ProjectClusterRelation) error {
	if args == nil {
		return errors.New("nil projectClusterRelation info")
	}

	args.CreatedAt = time.Now().Unix()
	args.CreatedBy = "system"
	_, err := c.InsertOne(context.TODO(), args)

	return err
}
