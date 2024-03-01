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

type GuanceyunMonitor struct {
	CheckerName string `json:"checker_name"`
	CheckerID   string `json:"checker_id"`
}

func ListGuanceyunMonitor(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	contents, err := service.ListGuanceyunMonitor(c.Param("id"), c.Query("search"))
	if err != nil {
		ctx.Err = err
		return
	}

	resp := make([]GuanceyunMonitor, 0)
	for _, content := range contents {
		resp = append(resp, GuanceyunMonitor{
			CheckerName: content.JSONScript.Name,
			CheckerID:   content.UUID,
		})
	}
	ctx.Resp = resp
}
