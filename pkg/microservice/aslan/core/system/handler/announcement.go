package handler

import (
	"github.com/gin-gonic/gin"

	systemmodel "github.com/koderover/zadig/pkg/microservice/aslan/core/system/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CreateAnnouncement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(systemmodel.Announcement)

	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid notify args")
		return
	}
	ctx.Err = service.CreateAnnouncement(ctx.Username, args, ctx.Logger)
}

func UpdateAnnouncement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(systemmodel.Announcement)

	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid notify args")
		return
	}
	ctx.Err = service.UpdateAnnouncement(ctx.Username, args.ID.Hex(), args, ctx.Logger)
}

func PullAllAnnouncement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.PullAllAnnouncement(ctx.Username, ctx.Logger)
}

func PullNotifyAnnouncement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.PullNotifyAnnouncement(ctx.Username, ctx.Logger)
}

func DeleteAnnouncement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ID := c.Param("id")

	ctx.Err = service.DeleteAnnouncement(ctx.Username, ID, ctx.Logger)
}
