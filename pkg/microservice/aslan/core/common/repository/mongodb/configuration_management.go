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
	"fmt"
	"time"

	"github.com/pkg/errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"github.com/koderover/zadig/v2/pkg/tool/nacos"
)

type ConfigurationManagementColl struct {
	*mongo.Collection

	coll string
}

func NewConfigurationManagementColl() *ConfigurationManagementColl {
	name := models.ConfigurationManagement{}.TableName()
	return &ConfigurationManagementColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ConfigurationManagementColl) GetCollectionName() string {
	return c.coll
}

func (c *ConfigurationManagementColl) EnsureIndex(ctx context.Context) error {
	_, _ = c.Indexes().DropOne(ctx, "server_address_1")
	return nil
}

func (c *ConfigurationManagementColl) Create(ctx context.Context, args *models.ConfigurationManagement) error {
	if args == nil {
		return errors.New("configuration management is nil")
	}
	args.UpdateTime = time.Now().Unix()

	_, err := c.InsertOne(ctx, args)
	return err
}

func (c *ConfigurationManagementColl) List(ctx context.Context, _type string) ([]*models.ConfigurationManagement, error) {
	resp := make([]*models.ConfigurationManagement, 0)
	query := bson.M{}
	if _type != "" {
		query = bson.M{"type": _type}
	}
	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	return resp, cursor.All(ctx, &resp)
}

func (c *ConfigurationManagementColl) ListNacos(ctx context.Context) ([]*models.ConfigurationManagement, error) {
	resp := make([]*models.ConfigurationManagement, 0)
	query := bson.M{"type": bson.M{"$in": bson.A{setting.SourceFromNacos, setting.SourceFromNacosEEMSE}}}
	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	return resp, cursor.All(ctx, &resp)
}

func (c *ConfigurationManagementColl) GetByID(ctx context.Context, idString string) (*models.ConfigurationManagement, error) {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return nil, err
	}
	query := bson.M{"_id": id}

	resp := new(models.ConfigurationManagement)
	return resp, c.FindOne(ctx, query).Decode(resp)
}

func (c *ConfigurationManagementColl) GetBySystemIdentity(systemIdentity string) (*models.ConfigurationManagement, error) {
	configManagement := &models.ConfigurationManagement{}
	query := bson.M{"system_identity": systemIdentity}
	if err := c.Collection.FindOne(context.TODO(), query).Decode(configManagement); err != nil {
		return nil, err
	}
	return configManagement, nil
}

func (c *ConfigurationManagementColl) GetApolloByID(ctx context.Context, idString string) (*models.ApolloConfig, error) {
	info, err := c.GetByID(ctx, idString)
	if err != nil {
		return nil, err
	}
	if info.Type != setting.SourceFromApollo {
		return nil, errors.Errorf("unexpected apollo config type %s", info.Type)
	}
	apollo := &models.ApolloAuthConfig{}
	err = models.IToi(info.AuthConfig, apollo)
	if err != nil {
		return nil, errors.Wrap(err, "IToi")
	}
	return &models.ApolloConfig{
		ServerAddress:    info.ServerAddress,
		ApolloAuthConfig: apollo,
	}, nil
}

func (c *ConfigurationManagementColl) GetNacosByID(ctx context.Context, idString string) (*nacos.NacosConfig, error) {
	info, err := c.GetByID(ctx, idString)
	if err != nil {
		return nil, err
	}
	if info.Type != setting.SourceFromNacos && info.Type != setting.SourceFromNacos3DotX && info.Type != setting.SourceFromNacosEEMSE {
		return nil, errors.Errorf("unexpected nacos config type %s", info.Type)
	}
	var nacosConfig interface{}
	switch info.Type {
	case setting.SourceFromNacos:
		nacosConfig = &nacos.NacosAuthConfig{}
	case setting.SourceFromNacos3DotX:
		nacosConfig = &nacos.NacosAuthConfig{}
	case setting.SourceFromNacosEEMSE:
		nacosConfig = &nacos.NacosEEMSEAuthConfig{}
	}
	err = models.IToi(info.AuthConfig, nacosConfig)
	if err != nil {
		return nil, errors.Wrap(err, "IToi")
	}
	return &nacos.NacosConfig{
		Type:            info.Type,
		ServerAddress:   info.ServerAddress,
		NacosAuthConfig: nacosConfig,
	}, nil
}

func (c *ConfigurationManagementColl) Update(ctx context.Context, idString string, obj *models.ConfigurationManagement) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return fmt.Errorf("invalid id")
	}

	obj.UpdateTime = time.Now().Unix()
	filter := bson.M{"_id": id}
	update := bson.M{"$set": obj}

	_, err = c.UpdateOne(ctx, filter, update)
	return err
}

func (c *ConfigurationManagementColl) DeleteByID(ctx context.Context, idString string) error {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(ctx, query)
	return err
}
