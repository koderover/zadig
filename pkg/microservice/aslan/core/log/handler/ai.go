package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/log/service/ai"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func AIAnalyzeBuildLog(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("failed to get raw data")
		return
	}

	args := new(ai.BuildLogAnalysisArgs)
	args.Log = string(data)
	ctx.Resp, ctx.Err = ai.AnalyzeBuildLog(args, c.Query("projectName"), c.Param("workflowName"), c.Param("jobName"), taskID, ctx.Logger)
}
