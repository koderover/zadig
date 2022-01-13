package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func CreatePolicyDefine(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	policyDefine := new(models.PolicyDefine)
	if err := c.ShouldBindJSON(policyDefine); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.CreatePolicyDefine(policyDefine)
}
