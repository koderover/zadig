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
	"encoding/base64"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/pkg/oauth"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"net/http"
	"strconv"
)

func CreateCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	rep := new(models.CodeHost)
	if err := c.ShouldBindJSON(rep); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.CreateCodeHost(rep, ctx.Logger)
}

func ListCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service.List(c.Query("address"), c.Query("owner"), c.Query("source"), ctx.Logger)
}

func DeleteCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Err = service.DeleteCodeHost(id, ctx.Logger)
}

func GetCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.GetCodeHost(id, ctx.Logger)
}

type State struct {
	CodeHostID  int    `json:"code_host_id"`
	RedirectURL string `json:"redirect_url"`
}

type AuthArgs struct {
	RedirectURI string `json:"redirect_uri"`
	HostName string `json:"host_name"`
	ClientID string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Provider string `json:"provider"`
}

func AuthCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	var au AuthArgs
	if err := c.ShouldBindJSON(&au);err !=nil {
		ctx.Err = err
		return
	}
	idInt, err := strconv.Atoi(id)
	if err != nil {
		ctx.Err = err
		return
	}
	codeHost, err := service.GetCodeHost(idInt, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}
	oauth := oauth.Factory(au.Provider,au.RedirectURI, au.ClientID, au.ClientSecret, au.HostName)
	stateStruct := State{
		CodeHostID:  codeHost.ID,
		RedirectURL: "",
	}
	bs, err := json.Marshal(stateStruct)
	if err != nil {
		ctx.Err = err
		return
	}
	c.Redirect(http.StatusFound, oauth.LoginURL(base64.URLEncoding.EncodeToString(bs)))
}

func Callback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	stateStr := c.Query("state")
	bs, err := base64.URLEncoding.DecodeString(stateStr)
	if err != nil {
		ctx.Err = err
		return
	}
	var state State
	if err := json.Unmarshal(bs, &state); err != nil {
		c.Redirect(http.StatusFound,state.RedirectURL)
		return
	}
	codehost , err := service.GetCodeHost(state.CodeHostID,ctx.Logger)
	if err != nil {
		c.Redirect(http.StatusFound,state.RedirectURL)
		return
	}
	o:= oauth.Factory(codehost.Type,state.RedirectURL, codehost.ApplicationId, codehost.ClientSecret, codehost.Address)
	token , err := o.HandleCallback(c.Request)
	if err !=nil {
		c.Redirect(http.StatusFound,state.RedirectURL)
		return
	}
	codehost.AccessToken = token.AccessToken
	codehost.RefreshToken = token.RefreshToken
	if _,err := service.UpdateCodeHostByToken(codehost,ctx.Logger);err !=nil {
		c.Redirect(http.StatusFound,state.RedirectURL)
		return
	}
}

func UpdateCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.Err = err
		return
	}
	req := &models.CodeHost{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	req.ID = id
	ctx.Resp, ctx.Err = service.UpdateCodeHost(req, ctx.Logger)
}
