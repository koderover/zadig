package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CreatePolicyDefine(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	policyDefine := new(models.PolicyDefine)
	if err := c.ShouldBindJSON(policyDefine); err != nil {
		ctx.Err = err
		return
	}

	policyDefine.CreateBy = ctx.UserName
	ctx.Err = service.CreatePolicyDefine(policyDefine)
}

func ListPolicyDefine(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}

	ctx.Resp, ctx.Err = service.ListPolicyDefine(projectName)
}

func DeletePolicyDefine(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	if id == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("id can not be empty")
	}
	ctx.Err = service.DeletePolicyDefine(id)
}
