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

package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func ListHelmRepo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Logger.Infof("ListHelmRepo start")
	ctx.Resp, ctx.Err = commonservice.ListHelmRepo()
	respj, _ := json.Marshal(ctx.Resp)
	ctx.Logger.Infof("resp:%s", respj)

}
