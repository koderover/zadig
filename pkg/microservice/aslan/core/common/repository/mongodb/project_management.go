/*
 * Copyright 2022 The KodeRover Authors.
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

package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ProjectManagementColl struct {
	*mongo.Collection

	coll string
}

func NewProjectManagementColl() *ProjectManagementColl {
	name := models.ProjectManagement{}.TableName()
	coll := &ProjectManagementColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *ProjectManagementColl) GetCollectionName() string {
	return c.coll
}
func (c *ProjectManagementColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *ProjectManagementColl) Create(pm *models.ProjectManagement) error {
	pm.UpdatedAt = time.Now().Unix()
	_, err := c.Collection.InsertOne(context.TODO(), pm)
	if err != nil {
		log.Error("repository Create err : %v", err)
		return err
	}
	return nil
}

func (c *ProjectManagementColl) GetByID(idHex string) (*models.ProjectManagement, error) {
	pm := &models.ProjectManagement{}
	id, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return nil, err
	}
	query := bson.M{"_id": id}

	err = c.FindOne(context.Background(), query).Decode(pm)
	return pm, err
}

func (c *ProjectManagementColl) UpdateByID(idHex string, pm *models.ProjectManagement) error {
	id, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return fmt.Errorf("invalid id")
	}

	pm.UpdatedAt = time.Now().Unix()
	filter := bson.M{"_id": id}
	update := bson.M{"$set": pm}

	_, err = c.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		log.Error("repository UpdateByID err : %v", err)
		return err
	}
	return nil
}

func (c *ProjectManagementColl) DeleteByID(idHex string) error {
	id, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(context.Background(), query)
	return err
}

func (c *ProjectManagementColl) List() ([]*models.ProjectManagement, error) {
	query := bson.M{}
	resp := make([]*models.ProjectManagement, 0)

	cursor, err := c.Collection.Find(context.Background(), query)
	if err != nil {
		return nil, err
	}

	return resp, cursor.All(context.Background(), &resp)
}

// Deprecated since 1.19.0
func (c *ProjectManagementColl) GetJira() (*models.ProjectManagement, error) {
	jira := &models.ProjectManagement{}
	query := bson.M{"type": setting.ProjectManagementTypeJira}

	err := c.Collection.FindOne(context.TODO(), query).Decode(jira)
	if err != nil {
		return nil, err
	}
	return jira, nil
}

func (c *ProjectManagementColl) GetJiraByID(idHex string) (*models.ProjectManagement, error) {
	id, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return nil, err
	}
	jira := &models.ProjectManagement{}
	query := bson.M{"_id": id, "type": setting.ProjectManagementTypeJira}

	err = c.Collection.FindOne(context.TODO(), query).Decode(jira)
	if err != nil {
		return nil, err
	}
	return jira, nil
}

// Deprecated since 1.19.0
func (c *ProjectManagementColl) GetMeego() (*models.ProjectManagement, error) {
	meego := &models.ProjectManagement{}
	query := bson.M{"type": setting.ProjectManagementTypeMeego}

	err := c.Collection.FindOne(context.TODO(), query).Decode(meego)
	if err != nil {
		return nil, err
	}
	return meego, nil
}

func (c *ProjectManagementColl) GetMeegoByID(idHex string) (*models.ProjectManagement, error) {
	id, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return nil, err
	}
	meego := &models.ProjectManagement{}
	query := bson.M{"_id": id, "type": setting.ProjectManagementTypeMeego}

	err = c.Collection.FindOne(context.TODO(), query).Decode(meego)
	if err != nil {
		return nil, err
	}
	return meego, nil
}

func (c *ProjectManagementColl) GetPingCodeByID(idHex string) (*models.ProjectManagement, error) {
	id, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return nil, err
	}
	pingcode := &models.ProjectManagement{}
	query := bson.M{"_id": id, "type": setting.ProjectManagementTypePingCode}

	err = c.Collection.FindOne(context.TODO(), query).Decode(pingcode)
	if err != nil {
		return nil, err
	}
	return pingcode, nil
}

func (c *ProjectManagementColl) GetTapdByID(idHex string) (*models.ProjectManagement, error) {
	id, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return nil, err
	}
	tapd := &models.ProjectManagement{}
	query := bson.M{"_id": id, "type": setting.ProjectManagementTypeTapd}

	err = c.Collection.FindOne(context.TODO(), query).Decode(tapd)
	if err != nil {
		return nil, err
	}
	return tapd, nil
}

func (c *ProjectManagementColl) GetBySystemIdentity(systemIdentity string) (*models.ProjectManagement, error) {
	projectManagement := &models.ProjectManagement{}
	query := bson.M{"system_identity": systemIdentity}
	if err := c.Collection.FindOne(context.TODO(), query).Decode(projectManagement); err != nil {
		return nil, err
	}
	return projectManagement, nil
}

func (c *ProjectManagementColl) GetJiraSpec(id string) (*models.ProjectManagementJiraSpec, error) {
	info, err := c.GetJiraByID(id)
	if err != nil {
		return nil, err
	}

	spec := &models.ProjectManagementJiraSpec{}
	err = models.IToi(info.Spec, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to convert jira spec, err: %v", err)
	}
	return spec, nil
}

func (c *ProjectManagementColl) GetPingCodeSpec(id string) (*models.ProjectManagementPingCodeSpec, error) {
	pingcodeInfo, err := c.GetPingCodeByID(id)
	if err != nil {
		fmtErr := fmt.Errorf("failed to get pingcode info, err: %s", err)
		log.Error(fmtErr)
		return nil, fmtErr
	}

	spec := &models.ProjectManagementPingCodeSpec{}
	err = models.IToi(pingcodeInfo.Spec, spec)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

func (c *ProjectManagementColl) GetMeegoSpec(id string) (*models.ProjectManagementMeegoSpec, error) {
	meegoInfo, err := c.GetMeegoByID(id)
	if err != nil {
		return nil, err
	}

	spec := &models.ProjectManagementMeegoSpec{}
	err = models.IToi(meegoInfo.Spec, spec)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

func (c *ProjectManagementColl) GetTapdSpec(id string) (*models.ProjectManagementTapdSpec, error) {
	pingcodeInfo, err := c.GetTapdByID(id)
	if err != nil {
		fmtErr := fmt.Errorf("failed to get pingcode info, err: %s", err)
		log.Error(fmtErr)
		return nil, fmtErr
	}

	spec := &models.ProjectManagementTapdSpec{}
	err = models.IToi(pingcodeInfo.Spec, spec)
	if err != nil {
		return nil, err
	}

	return spec, nil
}
