/*
 * Copyright 2023 The KodeRover Authors.
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

package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

type GrafanaAlert struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
}

func ListGrafanaAlert(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	contents, err := service.ListGrafanaAlert(c.Param("id"))
	if err != nil {
		ctx.Err = err
		return
	}

	resp := make([]GrafanaAlert, 0)
	for _, content := range contents {
		resp = append(resp, GrafanaAlert{
			Name: content.Title,
			UID:  content.UID,
		})
	}
	ctx.Resp = resp
}
