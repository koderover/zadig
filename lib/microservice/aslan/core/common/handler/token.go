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

package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/lib/setting"
)

func GetToken(c *gin.Context) {
	authorization := c.Request.Header.Get(setting.HEADERAUTH)
	if authorization != "" {
		if strings.HasPrefix(authorization, setting.USERAPIKEY+" ") {
			token := strings.Split(authorization, " ")[1]
			c.JSON(http.StatusOK, gin.H{"token": token})
			return
		}
	}

	c.JSON(http.StatusBadRequest, gin.H{"message": "illegal request"})
}
