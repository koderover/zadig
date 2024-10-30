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
	"fmt"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func HandleCallback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &commonmodels.CallbackRequest{}
	err := c.ShouldBindJSON(req)
	if err != nil {
		log.Errorf("failed to validate callback request, the error is: %s", err)
		ctx.RespErr = fmt.Errorf("failed to validate callback request, the error is: %s", err)
		return
	}

	ctx.RespErr = commonservice.HandleCallback(req)

}

// @summary Webhook Notify
// @description Webhook Notify
// @tags 	webhook
// @accept 	json
// @produce json
// @Param 	body 			body 		webhooknotify.WebHookNotify         true 	"body"
// @success 200
// @router /api/aslan/webhook/test [post]
func WebhookNotifyTest(c *gin.Context) {
	// ctx := internalhandler.NewContext(c)
	// defer func() { internalhandler.JSONResponse(c, ctx) }()

	// // 读取请求体
	// bodyBytes, err := ioutil.ReadAll(c.Request.Body)
	// if err != nil {
	// 	ctx.RespErr = fmt.Errorf("Failed to read request body")
	// 	return
	// }

	// log.Debugf("Request Header: %+v", c.Request.Header)
	// log.Debugf("Request Body: %v", string(bodyBytes))
}

// @summary Workflow Webhook Notify Build Job Spec
// @description Workflow Webhook Build Job Spec
// @tags 	webhook
// @accept 	json
// @produce json
// @Param 	body 			body 		webhooknotify.WorkflowNotifyJobTaskBuildSpec 		true 	"body"
// @success 200
// @router /api/aslan/webhook/notify/buildJobSpec [post]
func WebhookNotifyBuildJobSpec(c *gin.Context) {
}

// @summary Workflow Webhook Notify Deploy Job Spec
// @description Workflow Webhook Deploy Job Spec
// @tags 	webhook
// @accept 	json
// @produce json
// @Param 	body 			body 		webhooknotify.WorkflowNotifyJobTaskDeploySpec 		true 	"body"
// @success 200
// @router /api/aslan/webhook/notify/deployJobSpec [post]
func WebhookNotifyDeployJobSpec(c *gin.Context) {
}
