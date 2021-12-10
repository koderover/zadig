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
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
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

func AuthCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	provider := c.Query("provider")
	redirect := c.Query("redirect")
	redirectHost, err := url.Parse(redirect)
	if err != nil {
		ctx.Err = err
		return
	}
	id := c.Param("id")
	idInt, err := strconv.Atoi(id)
	if err != nil {
		ctx.Err = err
		return
	}
	authStateString := fmt.Sprintf("%s%s%s%s%s", redirect, "&codeHostId=", id, "&provider=", provider)
	codeHost, err := service.GetCodeHost(idInt, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}
	callBackUrl := fmt.Sprintf("%s://%s%s", redirectHost.Scheme, redirectHost.Host, "/api/directory/codehosts/callback")

	var authUrl string
	var tokenUrl string
	var scopes []string
	if provider == "gitlab" {
		scopes = []string{"api", "read_user"}
		authUrl = codeHost.Address + "/oauth/authorize"
		tokenUrl = codeHost.Address + "/oauth/token"
	} else if provider == "github" {
		scopes = []string{"repo", "user"}
		authUrl = codeHost.Address + "/login/oauth/authorize"
		tokenUrl = codeHost.Address + "/login/oauth/access_token"
	}
	authConfig := &oauth2.Config{
		ClientID:     codeHost.ApplicationId,
		ClientSecret: codeHost.ClientSecret,
		RedirectURL:  callBackUrl,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authUrl,
			TokenURL: tokenUrl,
		},
		Scopes: scopes,
	}
	redirectURL := authConfig.AuthCodeURL(authStateString)
	c.Redirect(http.StatusFound, redirectURL)
}

func Callback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	state := c.Query("state")
	state , err := url.PathUnescape(state)
	if err != nil {
		ctx.Logger.Errorf("code_host_get_call_back PathUnescape state:%s err : %s", state, err)
		url := fmt.Sprintf("%s%s", state, "&errMessage=failed_to_parse_redirect_url")
		c.Redirect(http.StatusFound,url)
		return
	}
	urlArray := strings.Split(state, "&codeHostId=")
	frontEndUrl := urlArray[0]
	if len(urlArray) != 2 {
		ctx.Logger.Errorf("code_host_get_call_back split &codeHostId fail ,state: %s", state)
		url := fmt.Sprintf("%s?%s", frontEndUrl, "&errMessage=failed_to_parse_redirect_url")
		c.Redirect(http.StatusFound,url)
		return
	}
	if strings.Contains(frontEndUrl, "errCode") {
		frontEndUrl = strings.Split(frontEndUrl, "?errCode")[0]
	}

	gitlabError := c.Query("error")
	if gitlabError != "" {
		ctx.Logger.Warnf("code_host_get_call_back_gitlab user denied err: %s", gitlabError)
		url := fmt.Sprintf("%s%s%s", frontEndUrl, "?", "&errMessage=access_denied")
		c.Redirect(http.StatusFound, url)
		return
	}
	stateURL, err := url.Parse(state)
	if err != nil {
		url := fmt.Sprintf("%s%s%s", frontEndUrl, "?", "&errMessage=failed_to_parse_redirect_url")
		http.Redirect(c.Writer, c.Request, url, http.StatusFound)
		return
	}
	codeHostArray := strings.Split(urlArray[1], "&provider=")
	codeHostID, err := strconv.Atoi(codeHostArray[0])
	if err != nil {
		ctx.Logger.Errorf("code_host_get_call_back_gitlab codeHostID convert err : %s", err)
		url := fmt.Sprintf("%s%s%s", frontEndUrl, "?", "&errMessage=codeHostID convert failed")
		c.Redirect(http.StatusFound, url)
		return
	}
	code := c.Query("code")
	iCodehost, err := service.GetCodeHost(codeHostID, ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("code_host_get_call_back_gitlab GetCodeHostByID  err: %s", err)
		url := fmt.Sprintf("%s%s%s", frontEndUrl, "?", "&errMessage=get codehost failed")
		c.Redirect(http.StatusFound, url)
		return
	}

	callBackUrl := fmt.Sprintf("%s://%s%s", stateURL.Scheme, stateURL.Host, "/api/directory/codehosts/callback")
	authConfig := &oauth2.Config{
		ClientID:     iCodehost.ApplicationId,
		ClientSecret: iCodehost.ClientSecret,
		RedirectURL:  callBackUrl,
		//RedirectURL: "http://localhost:34001/directory/codehosts/callback",
	}
	if codeHostArray[1] == "gitlab" {
		authConfig.Scopes = []string{"api", "read_user"}
		authConfig.Endpoint = oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s%s", iCodehost.Address, "/oauth/authorize"),
			TokenURL: fmt.Sprintf("%s%s", iCodehost.Address, "/oauth/token"),
		}
	} else if codeHostArray[1] == "github" {
		authConfig.Scopes = []string{"repo", "user"}
		authConfig.Endpoint = oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s%s", iCodehost.Address, "/login/oauth/authorize"),
			TokenURL: fmt.Sprintf("%s%s", iCodehost.Address, "/login/oauth/access_token"),
		}
	}
	dc := http.DefaultClient
	ctxx := context.WithValue(context.Background(), oauth2.HTTPClient, dc)
	token, err := authConfig.Exchange(ctxx, code)
	if err != nil {
		ctx.Logger.Errorf("code_host_get_call_back_gitlab Exchange  err: %s", err)
		url := fmt.Sprintf("%s%s%s", frontEndUrl, "?", "&errMessage=exchange failed")
		c.Redirect(http.StatusFound, url)
		return
	}

	iCodehost.AccessToken = token.AccessToken
	iCodehost.RefreshToken = token.RefreshToken
	ctx.Logger.Infof("%+v", iCodehost)
	_, err = service.UpdateCodeHostByToken(iCodehost, ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("UpdateCodeHostByToken err: %s", err)
		url := fmt.Sprintf("%s%s%s", frontEndUrl, "?", "&errMessage=update codehost failed")
		c.Redirect(http.StatusFound, url)
		return
	}

	//redirect to front end
	redirectURL := ""
	if strings.Contains(frontEndUrl, "?succeed") {
		redirectURL = fmt.Sprintf("%s%s", strings.Split(frontEndUrl, "?succeed")[0], "?succeed=true")
	} else {
		redirectURL = fmt.Sprintf("%s%s", frontEndUrl, "?succeed=true")
	}
	c.Redirect(http.StatusFound, redirectURL)

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
