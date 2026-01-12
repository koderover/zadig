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
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type InstallColl struct {
	*mongo.Collection

	coll string
}

func NewInstallColl() *InstallColl {
	name := models.Install{}.TableName()
	return &InstallColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *InstallColl) GetCollectionName() string {
	return c.coll
}

func (c *InstallColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
			bson.E{Key: "version", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *InstallColl) Find(name, version string) (*models.Install, error) {
	query := bson.M{"name": name, "version": version}
	install := new(models.Install)
	err := c.FindOne(context.TODO(), query).Decode(install)
	return install, err
}

func (c *InstallColl) Create(args *models.Install) error {
	if args == nil {
		return errors.New("nil Install")
	}

	args.UpdateTime = time.Now().Unix()
	_, err := c.InsertOne(context.TODO(), args)

	return err
}

func (c *InstallColl) List() ([]*models.Install, error) {
	query := bson.M{}
	ctx := context.Background()
	resp := make([]*models.Install, 0)

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *InstallColl) Update(name, version string, args *models.Install) error {
	if args == nil {
		return errors.New("nil Install")
	}

	query := bson.M{"name": name, "version": version}
	change := bson.M{"$set": bson.M{
		"name":          args.Name,
		"version":       args.Version,
		"download_path": args.DownloadPath,
		"update_by":     args.UpdateBy,
		"update_time":   time.Now().Unix(),
		"scripts":       args.Scripts,
		"env":           args.Envs,
		"bin_path":      args.BinPath,
		"enabled":       args.Enabled,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *InstallColl) UpdateSystemDefault(name, version string, installInfoPreset map[string]*models.Install) error {
	packageKey := fmt.Sprintf("%s-%s", name, version)

	installInfo, ok := installInfoPreset[packageKey]
	if !ok {
		return nil
	}

	oid, err := primitive.ObjectIDFromHex(installInfo.ObjectIDHex)
	if err != nil {
		return err
	}
	query := bson.M{"_id": oid}

	change := bson.M{"$set": bson.M{
		"name":          installInfo.Name,
		"version":       installInfo.Version,
		"download_path": installInfo.DownloadPath,
		"update_by":     installInfo.UpdateBy,
		"update_time":   time.Now().Unix(),
		"scripts":       installInfo.Scripts,
		"env":           installInfo.Envs,
		"bin_path":      installInfo.BinPath,
		"enabled":       installInfo.Enabled,
	}}

	_, err = c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *InstallColl) Delete(name, version string) error {
	query := bson.M{"name": name, "version": version}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *InstallColl) initData(installInfoPreset map[string]*models.Install) {
	for _, installInfo := range installInfoPreset {
		pkgInfo := &models.Install{
			Name:         installInfo.Name,
			Version:      installInfo.Version,
			Scripts:      installInfo.Scripts,
			UpdateTime:   time.Now().Unix(),
			UpdateBy:     installInfo.UpdateBy,
			Envs:         installInfo.Envs,
			BinPath:      installInfo.BinPath,
			Enabled:      installInfo.Enabled,
			DownloadPath: installInfo.DownloadPath,
		}

		oid, err := primitive.ObjectIDFromHex(installInfo.ObjectIDHex)
		if err != nil {
			continue
		}
		query := bson.M{"_id": oid}
		change := bson.M{"$set": pkgInfo}

		_, _ = c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	}
}

func (c *InstallColl) InitInstallData(installInfoPreset map[string]*models.Install) error {
	log := log.SugaredLogger()
	installData, err := c.List()
	if err != nil {
		return err
	}
	if len(installData) == 0 {
		c.initData(installInfoPreset)
	} else {
		for _, installs := range installData {
			if installs.UpdateBy != setting.SystemUser {
				continue
			}
			err := c.UpdateSystemDefault(installs.Name, installs.Version, installInfoPreset)
			if err != nil {
				log.Errorf("failed to initialize package: %s, the error is: %v", installs.Name, err)
			}
		}
	}

	return nil
}
