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

package orm

import (
	"gorm.io/gorm"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/repository/models"
	gormtool "github.com/koderover/zadig/pkg/tool/gorm"
)

type ConnectorColl struct {
	*gorm.DB

	coll string
}

//func init() {
//	name := models.Connector{}.TableName()
//	ret := &ConnectorColl{
//		DB:   gormtool.DB(config.MysqlDexDB()),
//		coll: name,
//	}
//
//	err := ret.AutoMigrate(&models.Connector{})
//	if err != nil {
//		panic(err)
//	}
//}

func NewConnectorColl() *ConnectorColl {
	name := models.Connector{}.TableName()
	return &ConnectorColl{
		DB:   gormtool.DB(config.MysqlDexDB()),
		coll: name,
	}
}

func (c *ConnectorColl) GetCollectionName() string {
	return c.coll
}

func (c *ConnectorColl) List() ([]*models.Connector, error) {
	var res []*models.Connector
	result := c.Find(&res)

	return res, result.Error
}

func (c *ConnectorColl) Get(id string) (*models.Connector, error) {
	res := &models.Connector{}
	result := c.First(&res, "id=?", id)

	return res, result.Error
}

func (c *ConnectorColl) Create(obj *models.Connector) error {
	obj.ResourceVersion = "1"
	result := c.DB.Create(obj)

	return result.Error
}

func (c *ConnectorColl) Update(obj *models.Connector) error {
	current, err := c.Get(obj.ID)
	if err != nil {
		return err
	}

	obj.ResourceVersion = current.ResourceVersion
	obj.IncreaseResourceVersion()
	result := c.DB.Save(obj)

	return result.Error
}

func (c *ConnectorColl) Delete(id string) error {
	result := c.DB.Delete(&models.Connector{ID: id})

	return result.Error
}
