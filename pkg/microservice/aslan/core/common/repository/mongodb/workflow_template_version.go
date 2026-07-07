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

package mongodb

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"github.com/koderover/zadig/v2/pkg/util"
)

type WorkflowV4TemplateVersionColl struct {
	*mongo.Collection

	coll string
}

type WorkflowTemplateVersionQueryOption struct {
	TemplateID string
	Version    int
	VersionID  string
}

func NewWorkflowV4TemplateVersionColl() *WorkflowV4TemplateVersionColl {
	name := models.WorkflowV4TemplateVersion{}.TableName()
	return &WorkflowV4TemplateVersionColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *WorkflowV4TemplateVersionColl) GetCollectionName() string {
	return c.coll
}

func (c *WorkflowV4TemplateVersionColl) EnsureIndex(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "template_id", Value: 1},
				bson.E{Key: "version", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				bson.E{Key: "template_name", Value: 1},
			},
		},
	}
	_, err := c.Indexes().CreateMany(ctx, indexes, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *WorkflowV4TemplateVersionColl) Create(obj *models.WorkflowV4TemplateVersion) (string, error) {
	if obj == nil {
		return "", fmt.Errorf("nil object")
	}
	obj.ID = primitive.NilObjectID
	if obj.CreateTime == 0 {
		obj.CreateTime = time.Now().Unix()
	}
	res, err := c.InsertOne(context.TODO(), obj)
	if err != nil {
		return "", err
	}
	id, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", fmt.Errorf("failed to get object id from create")
	}
	return id.Hex(), nil
}

func (c *WorkflowV4TemplateVersionColl) CreateNext(template *models.WorkflowV4Template, user string) (*models.WorkflowV4TemplateVersion, error) {
	if template == nil {
		return nil, fmt.Errorf("nil template")
	}
	latest, err := c.GetLatest(template.ID.Hex())
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}
	nextVersion := 1
	if latest != nil {
		nextVersion = latest.Version + 1
	}

	snapshot := new(models.WorkflowV4Template)
	if err := util.DeepCopy(snapshot, template); err != nil {
		return nil, err
	}
	snapshot.LatestVersion = nextVersion
	snapshot.LatestVersionID = ""

	version := &models.WorkflowV4TemplateVersion{
		TemplateID:   template.ID.Hex(),
		TemplateName: template.TemplateName,
		Version:      nextVersion,
		Snapshot:     snapshot,
		Hash:         calculateWorkflowTemplateHash(snapshot),
		CreatedBy:    user,
		CreateTime:   time.Now().Unix(),
	}

	id, err := c.Create(version)
	if err != nil {
		return nil, err
	}
	version.ID, _ = primitive.ObjectIDFromHex(id)
	return version, nil
}

func (c *WorkflowV4TemplateVersionColl) Find(opt *WorkflowTemplateVersionQueryOption) (*models.WorkflowV4TemplateVersion, error) {
	if opt == nil {
		return nil, fmt.Errorf("nil option")
	}
	query := bson.M{}
	if opt.VersionID != "" {
		id, err := primitive.ObjectIDFromHex(opt.VersionID)
		if err != nil {
			return nil, err
		}
		query["_id"] = id
	} else {
		query["template_id"] = opt.TemplateID
		if opt.Version > 0 {
			query["version"] = opt.Version
		}
	}

	resp := new(models.WorkflowV4TemplateVersion)
	if err := c.FindOne(context.TODO(), query).Decode(resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *WorkflowV4TemplateVersionColl) GetLatest(templateID string) (*models.WorkflowV4TemplateVersion, error) {
	resp := new(models.WorkflowV4TemplateVersion)
	query := bson.M{"template_id": templateID}
	opt := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})
	if err := c.FindOne(context.TODO(), query, opt).Decode(resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *WorkflowV4TemplateVersionColl) List(templateID string) ([]*models.WorkflowV4TemplateVersion, error) {
	resp := make([]*models.WorkflowV4TemplateVersion, 0)
	query := bson.M{"template_id": templateID}
	opt := options.Find().SetSort(bson.D{{Key: "version", Value: -1}})
	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, err
	}
	if err := cursor.All(context.TODO(), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func calculateWorkflowTemplateHash(template *models.WorkflowV4Template) string {
	bs, _ := json.Marshal(template)
	return fmt.Sprintf("%x", md5.Sum(bs))
}
