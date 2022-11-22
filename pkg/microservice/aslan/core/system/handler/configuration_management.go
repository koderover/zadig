package handler

import (
	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListConfigurationManagement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListConfigurationManagement(ctx.Logger)
}

func CreateConfigurationManagement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	var args commonmodels.ConfigurationManagement
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Err = service.CreateConfigurationManagement(&args, ctx.Logger)
}

func GetConfigurationManagement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetConfigurationManagement(c.Param("id"), ctx.Logger)
}

func UpdateConfigurationManagement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	var args commonmodels.ConfigurationManagement
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Err = service.UpdateConfigurationManagement(c.Param("id"), &args, ctx.Logger)
}

func DeleteConfigurationManagement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = service.DeleteConfigurationManagement(c.Param("id"), ctx.Logger)
}

func ValidateConfigurationManagement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	b, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Err = service.ValidateConfigurationManagement(string(b), ctx.Logger)
}
