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

package repository

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

type RepositoryCache struct {
	cache map[string]*models.Service
}

func NewRepsitoryCache() *RepositoryCache {
	return &RepositoryCache{
		cache: make(map[string]*models.Service),
	}
}

func (c *RepositoryCache) QueryTemplateServiceWithCache(option *mongodb.ServiceFindOption, production bool) (*models.Service, error) {
	env := "testing"
	if production {
		env = "production"
	}
	key := fmt.Sprintf("%s:%s:%s:%d", option.ProductName, option.ServiceName, env, option.Revision)

	if service, ok := c.cache[key]; ok {
		return service, nil
	}

	service, err := QueryTemplateService(option, production)
	if err != nil {
		return nil, err
	}
	c.cache[key] = service

	return service, nil
}
