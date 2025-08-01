/*
Copyright 2025 The KodeRover Authors.

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
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type getSystemLanguageResp struct {
	Language string `json:"language"`
}

func GetSystemLanguage(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	lang, err := service.GetSystemLanguage(ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp = &getSystemLanguageResp{
		Language: lang,
	}
}

type setSystemLanguageArgs struct {
	Language string `json:"language"`
}

func SetSystemLanguage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateThemeInfo GetRawData err :%v", err)
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	languageSetting := &setSystemLanguageArgs{}
	if err := json.Unmarshal(args, languageSetting); err != nil {
		log.Errorf("update language setting Unmarshal err :%v", err)
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	if languageSetting == nil {
		return
	}

	ctx.RespErr = service.SetSystemLanguage(languageSetting.Language, ctx.Logger)
}
