/*
Copyright 2021 The KodeRover Authors.

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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/pkg/config"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type OrganizationColl struct {
	*mongo.Collection

	coll string
}

func NewOrganizationColl() *OrganizationColl {
	name := models.Organization{}.TableName()
	return &OrganizationColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *OrganizationColl) Get(id int) (*models.Organization, bool, error) {
	org := &models.Organization{}
	query := bson.M{"id": id}

	err := c.FindOne(context.TODO(), query).Decode(org)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, false, nil
		}
		return nil, false, err
	}
	return org, true, nil
}
