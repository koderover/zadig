package handler

import (
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
	ctx.Resp, ctx.Err = service.FindCodeHost(ctx.Logger)
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
	authStateString := fmt.Sprintf("%s%s%d%s%s", redirect, "&codeHostId=", id, "&provider=", provider)
	codeHost, err := service.GetCodeHost(idInt, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}
	callBackUrl := fmt.Sprintf("%s://%s%s", redirectHost.Scheme, redirectHost.Host, "/api/directory/codehosts/callback")
	authConfig := &oauth2.Config{
		ClientID:     codeHost.ApplicationId,
		ClientSecret: codeHost.ClientSecret,
		RedirectURL:  callBackUrl,
		//RedirectURL: "http://localhost:34001/directory/codehosts/callback",
	}
	redirectURL := authConfig.AuthCodeURL(authStateString)
	http.Redirect(c.Writer, c.Request, redirectURL, http.StatusFound)
}

func Callback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	//增加数据验证
	state := c.Query("state")
	urlArray := strings.Split(state, "&codeHostId=")
	frontEndUrl := urlArray[0]
	if strings.Contains(frontEndUrl, "errCode") {
		frontEndUrl = strings.Split(frontEndUrl, "?errCode")[0]
	}

	gitlabError := c.Query("error")
	if gitlabError != "" {
		ctx.Logger.Warn("view code_host_get_call_back_gitlab user denied")
		url := fmt.Sprintf("%s%s%d%s", frontEndUrl, "?errCode=", 10007, "&errMessage=access_denied")
		http.Redirect(c.Writer, c.Request, url, http.StatusFound)
		return
	}
	stateURL, err := url.Parse(state)
	if err != nil {
		url := fmt.Sprintf("%s%s%d%s", frontEndUrl, "?errCode=", 404, "&errMessage=failed_to_parse_redirect_url")
		http.Redirect(c.Writer, c.Request, url, http.StatusFound)
		return
	}
	codeHostArray := strings.Split(urlArray[1], "&provider=")
	codeHostID, err := strconv.Atoi(codeHostArray[0])
	if err != nil {
		ctx.Logger.Error("view code_host_get_call_back_gitlab codeHostID convert err : %v", err)
		url := fmt.Sprintf("%s%s%d%s", frontEndUrl, "?errCode=", 404, "&errMessage=codeHostID convert failed")
		http.Redirect(c.Writer, c.Request, url, http.StatusFound)
		return
	}
	code := c.Query("code")
	iCodehost, err := service.GetCodeHost(codeHostID, ctx.Logger)
	if err != nil {
		ctx.Logger.Error("view code_host_get_call_back_gitlab GetCodeHostByID  err : %v", err)
		url := fmt.Sprintf("%s%s%d%s", frontEndUrl, "?errCode=", 404, "&errMessage=get codehost failed")
		http.Redirect(c.Writer, c.Request, url, http.StatusFound)
		return
	}

	codehost := new(models.CodeHost)

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

	token, err := authConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		ctx.Logger.Error("view code_host_get_call_back_gitlab Exchange  err : %v", err)
		url := fmt.Sprintf("%s%s%d%s", frontEndUrl, "?errCode=", 10007, "&errMessage=exchange failed")
		http.Redirect(c.Writer, c.Request, url, http.StatusFound)
		return
	}

	codehost.AccessToken = token.AccessToken
	codehost.RefreshToken = token.RefreshToken
	_, err = service.UpdateCodeHost(codehost, ctx.Logger)
	if err != nil {
		ctx.Logger.Error("view UpdateCodeHostByToken err : %v", err)
		url := fmt.Sprintf("%s%s%d%s", frontEndUrl, "?errCode=", 404, "&errMessage=update codehost failed")
		http.Redirect(c.Writer, c.Request, url, http.StatusFound)
		return
	}

	//跳转回前端
	redirectURL := ""
	if strings.Contains(frontEndUrl, "?succeed") {
		redirectURL = fmt.Sprintf("%s%s", strings.Split(frontEndUrl, "?succeed")[0], "?succeed=true")
	} else {
		redirectURL = fmt.Sprintf("%s%s", frontEndUrl, "?succeed=true")
	}
	http.Redirect(c.Writer, c.Request, redirectURL, http.StatusFound)
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
