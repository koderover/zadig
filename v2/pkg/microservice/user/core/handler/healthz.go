/*
Copyright 2023 The KodeRover Authors.

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

package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/tool/log"

	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func Healthz(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Err = Health()
}

func Health() error {
	userDB, err := repository.DB.DB()
	if err != nil {
		log.Errorf("Healthz get db error:%s", err.Error())
		return err
	}
	if err := userDB.Ping(); err != nil {
		return err
	}

	dexDB, err := repository.DexDB.DB()
	if err != nil {
		log.Errorf("Healthz get dex db error:%s", err.Error())
		return err
	}

	return dexDB.Ping()
}
