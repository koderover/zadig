package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

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
