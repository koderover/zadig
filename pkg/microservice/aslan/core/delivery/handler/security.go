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
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	deliveryservice "github.com/koderover/zadig/pkg/microservice/aslan/core/delivery/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListDeliverySecurityStatistics(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	//params validate
	imageID := c.Query("imageId")
	if imageID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("imageId can't be empty!")
		return
	}

	ctx.Resp, ctx.Err = deliveryservice.FindDeliverySecurityStatistics(imageID, ctx.Logger)
}

func ListDeliverySecurity(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	//params validate
	imageID := c.Query("imageId")
	if imageID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("imageId can't be empty!")
		return
	}
	args := new(commonrepo.DeliverySecurityArgs)
	args.ImageID = imageID
	args.Severity = c.Query("severity")

	ctx.Resp, ctx.Err = deliveryservice.FindDeliverySecurity(args, ctx.Logger)
}

type DeliverySecurityInfo struct {
	Result            string                           `json:"result,omitempty"`
	DeliverySecuritys []*commonmodels.DeliverySecurity `json:"message,omitempty"`
}

func CreateDeliverySecurity(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	//params validate
	var deliverySecurityInfo DeliverySecurityInfo
	if err := c.ShouldBindWith(&deliverySecurityInfo, binding.JSON); err != nil {
		ctx.Logger.Info("ShouldBindWith err :%v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	deliverySecuritys := deliverySecurityInfo.DeliverySecuritys
	if len(deliverySecuritys) > 0 {
		errs := make([]string, 0)
		args := new(commonrepo.DeliverySecurityArgs)
		args.ImageID = deliverySecuritys[0].ImageID
		securitys, err := deliveryservice.FindDeliverySecurity(args, ctx.Logger)
		if err != nil {
			ctx.Logger.Error("FindDeliverySecurity err :%v", err)
			errs = append(errs, err.Error())
		}
		ctx.Resp = deliverySecuritys[0].ImageID
		if len(securitys) > 0 {
			ctx.Logger.Info("DeliverySecurity imageID : %s already exist!", deliverySecuritys[0].ImageID)
			return
		}

		for _, deliverySecurity := range deliverySecuritys {
			if deliverySecurity.ImageName != "" {
				err := deliveryservice.InsertDeliverySecurity(deliverySecurity, ctx.Logger)
				if err != nil {
					ctx.Logger.Error("InsertDeliverySecurity err :%v", err)
					errs = append(errs, err.Error())
				}
			}
		}

		if len(errs) != 0 {
			ctx.Err = e.NewHTTPError(500, strings.Join(errs, ","))
		}
	}
}
