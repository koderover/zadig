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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type CodehostColl struct {
	*mongo.Collection

	coll string
}

type ListArgs struct {
	IntegrationLevel setting.IntegrationLevel
	Project          string
	Owner            string
	Address          string
	Source           string
}

func NewCodehostColl() *CodehostColl {
	name := models.CodeHost{}.TableName()
	coll := &CodehostColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *CodehostColl) GetCollectionName() string {
	return c.coll
}
func (c *CodehostColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *CodehostColl) AddSystemCodeHost(iCodeHost *models.CodeHost) (*models.CodeHost, error) {
	iCodeHost.IntegrationLevel = setting.IntegrationLevelSystem
	return c.addCodeHost(iCodeHost)
}

func (c *CodehostColl) AddProjectCodeHost(projectName string, iCodeHost *models.CodeHost) (*models.CodeHost, error) {
	iCodeHost.IntegrationLevel = setting.IntegrationLevelProject
	iCodeHost.Project = projectName
	return c.addCodeHost(iCodeHost)
}

func (c *CodehostColl) addCodeHost(iCodeHost *models.CodeHost) (*models.CodeHost, error) {

	_, err := c.Collection.InsertOne(context.TODO(), iCodeHost)
	if err != nil {
		log.Error("repository AddCodeHost err : %v", err)
		return nil, err
	}
	return iCodeHost, nil
}

func (c *CodehostColl) GetCodeHostByAlias(alias string) (*models.CodeHost, error) {
	query := bson.M{
		"alias":      alias,
		"deleted_at": 0,
	}
	return c.getCodeHost(query)
}

func (c *CodehostColl) GetSystemCodeHostByAlias(alias string) (*models.CodeHost, error) {
	query := bson.M{
		"integration_level": setting.IntegrationLevelSystem,
		"alias":             alias,
		"deleted_at":        0,
	}
	return c.getCodeHost(query)
}

func (c *CodehostColl) GetProjectCodeHostByAlias(projectName, alias string) (*models.CodeHost, error) {
	query := bson.M{
		"integration_level": setting.IntegrationLevelProject,
		"project":           projectName,
		"alias":             alias,
		"deleted_at":        0,
	}
	return c.getCodeHost(query)
}

func (c *CodehostColl) GetProjectCodeHostByID(ID int, projectName string, ignoreDelete bool) (*models.CodeHost, error) {
	query := bson.M{
		"id":                ID,
		"integration_level": setting.IntegrationLevelProject,
		"project":           projectName,
	}
	if !ignoreDelete {
		query["deleted_at"] = 0
	}
	return c.getCodeHost(query)
}

func (c *CodehostColl) GetSystemCodeHostByID(ID int, ignoreDelete bool) (*models.CodeHost, error) {
	query := bson.M{
		"id":                ID,
		"integration_level": setting.IntegrationLevelSystem,
	}
	if !ignoreDelete {
		query["deleted_at"] = 0
	}
	return c.getCodeHost(query)
}

// Internal use only
func (c *CodehostColl) GetCodeHostByID(ID int, ignoreDelete bool) (*models.CodeHost, error) {
	query := bson.M{
		"id": ID,
	}
	if !ignoreDelete {
		query["deleted_at"] = 0
	}
	return c.getCodeHost(query)
}

func (c *CodehostColl) getCodeHost(query bson.M) (*models.CodeHost, error) {
	codehost := new(models.CodeHost)
	if err := c.Collection.FindOne(context.TODO(), query).Decode(codehost); err != nil {
		return nil, err
	}
	return codehost, nil
}

func (c *CodehostColl) List(args *ListArgs) ([]*models.CodeHost, error) {
	codeHosts := make([]*models.CodeHost, 0)
	query := bson.M{"deleted_at": 0}

	if args == nil {
		args = &ListArgs{}
	}
	if args.IntegrationLevel != "" {
		query["integration_level"] = args.IntegrationLevel
	}
	if args.Project != "" {
		query["project"] = args.Project
	}
	if args.Address != "" {
		query["address"] = args.Address
	}
	if args.Owner != "" {
		query["namespace"] = args.Owner
	}
	if args.Source != "" {
		query["type"] = args.Source
	}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &codeHosts)
	if err != nil {
		return nil, err
	}
	return codeHosts, nil
}

func (c *CodehostColl) AvailableCodeHost(projectName string) ([]*models.CodeHost, error) {
	codeHosts := make([]*models.CodeHost, 0)
	query := bson.M{
		"$or": bson.A{
			bson.M{"$and": bson.A{
				bson.M{"integration_level": setting.IntegrationLevelSystem},
				bson.M{"deleted_at": 0},
			}},
			bson.M{"$and": bson.A{
				bson.M{"integration_level": setting.IntegrationLevelProject},
				bson.M{"project": projectName},
				bson.M{"deleted_at": 0},
			}},
		},
	}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &codeHosts)
	if err != nil {
		return nil, err
	}
	return codeHosts, nil
}

func (c *CodehostColl) CodeHostList() ([]*models.CodeHost, error) {
	return c.codeHostList("")
}

func (c *CodehostColl) SystemCodeHostList() ([]*models.CodeHost, error) {
	return c.codeHostList(setting.IntegrationLevelSystem)
}

func (c *CodehostColl) ProjectCodeHostList() ([]*models.CodeHost, error) {
	return c.codeHostList(setting.IntegrationLevelProject)
}

func (c *CodehostColl) codeHostList(integrationLevel setting.IntegrationLevel) ([]*models.CodeHost, error) {
	codeHosts := make([]*models.CodeHost, 0)
	query := bson.M{}
	if integrationLevel != "" {
		query["integration_level"] = integrationLevel
	}
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &codeHosts)
	if err != nil {
		return nil, err
	}
	return codeHosts, nil
}

func (c *CodehostColl) DeleteProjectCodeHosts(projectName string) error {
	query := bson.M{
		"integration_level": setting.IntegrationLevelProject,
		"project":           projectName,
		"deleted_at":        0,
	}
	return c.deleteCodeHost(query)
}

func (c *CodehostColl) DeleteProjectCodeHostByID(projectName string, ID int) error {
	query := bson.M{
		"id":                ID,
		"integration_level": setting.IntegrationLevelProject,
		"project":           projectName,
		"deleted_at":        0,
	}
	return c.deleteCodeHost(query)
}

func (c *CodehostColl) DeleteSystemCodeHostByID(ID int) error {
	query := bson.M{
		"id":                ID,
		"integration_level": setting.IntegrationLevelSystem,
		"deleted_at":        0,
	}
	return c.deleteCodeHost(query)
}

func (c *CodehostColl) deleteCodeHost(query bson.M) error {
	change := bson.M{"$set": bson.M{
		"deleted_at": time.Now().Unix(),
	}}
	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Errorf("repository update fail,err:%s", err)
		return err
	}
	return nil
}

func (c *CodehostColl) UpdateCodeHost(host *models.CodeHost) (*models.CodeHost, error) {
	query := bson.M{
		"id":         host.ID,
		"deleted_at": 0,
	}

	return c.updateCodeHost(query, host)
}

func (c *CodehostColl) UpdateProjectCodeHost(projectName string, host *models.CodeHost) (*models.CodeHost, error) {
	query := bson.M{
		"id":                host.ID,
		"integration_level": setting.IntegrationLevelProject,
		"project":           projectName,
		"deleted_at":        0,
	}

	return c.updateCodeHost(query, host)
}

func (c *CodehostColl) updateCodeHost(query bson.M, host *models.CodeHost) (*models.CodeHost, error) {
	modifyValue := bson.M{
		"type":           host.Type,
		"address":        host.Address,
		"namespace":      host.Namespace,
		"application_id": host.ApplicationId,
		"client_secret":  host.ClientSecret,
		"region":         host.Region,
		"username":       host.Username,
		"password":       host.Password,
		"enable_proxy":   host.EnableProxy,
		"alias":          host.Alias,
		"updated_at":     time.Now().Unix(),
		"disable_ssl":    host.DisableSSL,
	}
	if host.Type == setting.SourceFromGerrit {
		modifyValue["access_token"] = host.AccessToken
	} else if host.Type == setting.SourceFromGitee || host.Type == setting.SourceFromGitlab || host.Type == setting.SourceFromGiteeEE {
		modifyValue["access_token"] = host.AccessToken
		modifyValue["refresh_token"] = host.RefreshToken
		modifyValue["updated_at"] = host.UpdatedAt
	} else if host.Type == setting.SourceFromOther {
		modifyValue["auth_type"] = host.AuthType
		modifyValue["ssh_key"] = host.SSHKey
		modifyValue["private_access_token"] = host.PrivateAccessToken
	}

	change := bson.M{"$set": modifyValue}
	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	return host, err
}

func (c *CodehostColl) UpdateCodeHostToken(host *models.CodeHost) (*models.CodeHost, error) {
	query := bson.M{"id": host.ID, "deleted_at": 0}
	change := bson.M{"$set": bson.M{
		"is_ready":      "2",
		"access_token":  host.AccessToken,
		"updated_at":    time.Now().Unix(),
		"refresh_token": host.RefreshToken,
	}}
	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	return host, err
}
