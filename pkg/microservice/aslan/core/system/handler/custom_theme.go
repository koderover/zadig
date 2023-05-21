package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func GetThemeInfos(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetThemeInfos(ctx.Logger)
}

func UpdateThemeInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateThemeInfo GetRawData err :%v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	theme := &models.Theme{}
	if err := json.Unmarshal(args, theme); err != nil {
		log.Errorf("UpdateThemeInfo Unmarshal err :%v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if theme == nil {
		return
	}

	ctx.Err = service.UpdateThemeInfo(theme, ctx.Logger)
}
