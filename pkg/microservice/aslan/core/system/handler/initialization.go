/*
Copyright 2023 The KodeRover Authors.

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
	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func GetSystemInitializationStatus(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetSystemInitializationStatus(ctx.Logger)
}

type InitializeUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Company  string `json:"company"`
	Email    string `json:"email"`
	Phone    int64  `json:"phone"`
	Reason   string `json:"reason"`
	Address  string `json:"address"`
}

func (req *InitializeUserReq) Validate() error {
	if len(req.Username) == 0 {
		return fmt.Errorf("username cannot be empty")
	}

	if len(req.Password) == 0 {
		return fmt.Errorf("password cannot be empty")
	}

	if !config.Enterprise() {
		if len(req.Company) == 0 {
			return fmt.Errorf("company cannot be empty")
		}

		if len(req.Email) == 0 {
			return fmt.Errorf("email cannot be empty")
		}

		if len(req.Reason) > 500 {
			return fmt.Errorf("reason is too long")
		}

		if len(req.Address) > 100 {
			return fmt.Errorf("address is too long")
		}
	}

	return nil
}

func InitializeUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(InitializeUserReq)
	if err := c.BindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid user initialization config")
		return
	}

	if err := req.Validate(); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = service.InitializeUser(req.Username, req.Password, req.Company, req.Email, req.Phone, req.Reason, req.Address, ctx.Logger)
}
