package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/tool/log"
)

func HandleCallback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &commonmodels.CallbackRequest{}
	err := c.ShouldBindJSON(req)
	if err != nil {
		log.Errorf("failed to validate callback request, the error is: %s", err)
		ctx.Err = fmt.Errorf("failed to validate callback request, the error is: %s", err)
		return
	}

	ctx.Err = commonservice.HandleCallback(req)

}
