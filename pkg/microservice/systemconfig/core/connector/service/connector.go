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

package service

import (
	"encoding/json"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/repository/orm"
	"github.com/koderover/zadig/pkg/shared/client/aslan"
	"github.com/koderover/zadig/pkg/tool/crypto"
)

func ListConnectorsInternal(logger *zap.SugaredLogger) ([]*Connector, error) {
	cs, err := orm.NewConnectorColl().List()
	if err != nil {
		logger.Errorf("Failed to list connectors, err: %s", err)
		return nil, err
	}

	var res []*Connector
	for _, c := range cs {
		cf := make(map[string]interface{})
		err = json.Unmarshal([]byte(c.Config), &cf)
		if err != nil {
			logger.Errorf("Failed to unmarshal config, err: %s", err)
			continue
		}
		res = append(res, &Connector{
			ConnectorBase: ConnectorBase{
				Type: ConnectorType(c.Type),
			},
			ID:     c.ID,
			Name:   c.Name,
			Config: cf,
		})
	}

	return res, nil
}

func ListConnectors(encryptedKey string, logger *zap.SugaredLogger) ([]*Connector, error) {
	aesKey, err := aslan.New(config.AslanServiceAddress()).GetTextFromEncryptedKey(encryptedKey)
	if err != nil {
		logger.Errorf("ListConnectors GetTextFromEncryptedKey, err: %s", err)
		return nil, err
	}
	cs, err := orm.NewConnectorColl().List()
	if err != nil {
		logger.Errorf("Failed to list connectors, err: %s", err)
		return nil, err
	}
	config, err := aslan.New(config.AslanServiceAddress()).GetDefaultLogin()
	if err != nil {
		logger.Errorf("Failed to list connectors, err: %s", err)
		return nil, err
	}
	var res []*Connector
	for _, c := range cs {
		cf := make(map[string]interface{})
		err = json.Unmarshal([]byte(c.Config), &cf)
		if err != nil {
			logger.Errorf("Failed to unmarshal config, err: %s", err)
			continue
		}
		if pw, ok := cf["bindPW"]; ok {
			cf["bindPW"], err = crypto.AesEncryptByKey(pw.(string), aesKey.PlainText)
			if err != nil {
				logger.Errorf("ListConnectors AesEncryptByKey, err: %s", err)
				return nil, err
			}
		}
		if clientSecret, ok := cf["clientSecret"]; ok {
			cf["clientSecret"], err = crypto.AesEncryptByKey(clientSecret.(string), aesKey.PlainText)
			if err != nil {
				logger.Errorf("ListConnectors AesEncryptByKey, err: %s", err)
				return nil, err
			}
		}
		isDefault := false
		if config.DefaultLogin == c.Type {
			isDefault = true
		}
		res = append(res, &Connector{
			ConnectorBase: ConnectorBase{
				Type: ConnectorType(c.Type),
			},
			ID:        c.ID,
			Name:      c.Name,
			Config:    cf,
			IsDefault: isDefault,
		})
	}

	return res, nil
}

func GetConnector(id string, logger *zap.SugaredLogger) (*Connector, error) {
	c, err := orm.NewConnectorColl().Get(id)
	if err != nil {
		logger.Errorf("Failed to get connector %s, err: %s", id, err)
		return nil, err
	}

	cf := make(map[string]interface{})
	err = json.Unmarshal([]byte(c.Config), &cf)
	if err != nil {
		logger.Warnf("Failed to unmarshal config, err: %s", err)

	}

	return &Connector{
		ConnectorBase: ConnectorBase{
			Type: ConnectorType(c.Type),
		},
		ID:     c.ID,
		Name:   c.Name,
		Config: cf,
	}, nil

}

func DeleteConnector(id string, _ *zap.SugaredLogger) error {
	return orm.NewConnectorColl().Delete(id)
}

func CreateConnector(ct *Connector, logger *zap.SugaredLogger) error {
	cf, err := json.Marshal(ct.Config)
	if err != nil {
		logger.Errorf("Failed to marshal config, err: %s", err)
		return err
	}

	obj := &models.Connector{
		ID:     ct.ID,
		Name:   ct.Name,
		Type:   string(ct.Type),
		Config: string(cf),
	}

	return orm.NewConnectorColl().Create(obj)
}

func UpdateConnector(ct *Connector, logger *zap.SugaredLogger) error {
	cf, err := json.Marshal(ct.Config)
	if err != nil {
		logger.Errorf("Failed to marshal config, err: %s", err)
		return err
	}

	obj := &models.Connector{
		ID:     ct.ID,
		Name:   ct.Name,
		Type:   string(ct.Type),
		Config: string(cf),
	}

	return orm.NewConnectorColl().Update(obj)
}
