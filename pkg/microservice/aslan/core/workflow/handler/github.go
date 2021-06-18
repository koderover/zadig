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
	"bytes"
	"io/ioutil"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-multierror"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/webhook"
	internalhandler "github.com/koderover/zadig/pkg/microservice/aslan/internal/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

// @Router /workflow/webhook/githubWebHook [POST]
// @Summary Process webhook for github
// @Accept  json
// @Produce json
// @Success 200 {object} map[string]string "map[string]string - {message: 'success information'}"
func ProcessGithub(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	log := ctx.Logger
	errs := &multierror.Error{}

	buf, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(e.ErrorMessage(e.ErrGithubWebHook.AddDesc(errs.Error())))
		c.Abort()
		return
	}

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
	// trigger classic pipeline
	output, err := webhook.ProcessGithubHook(c.Request, ctx.RequestID, log)
	if err != nil {
		log.Errorf("error happens to trigger classic pipeline %v", err)
		errs = multierror.Append(errs, err)
	}

	// trigger workflow
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
	err = webhook.ProcessGithubWebHook(c.Request, ctx.RequestID, log)

	if err != nil {
		log.Errorf("error happens to trigger workflow %v", err)
		errs = multierror.Append(errs, err)
	}

	if errs.ErrorOrNil() != nil {
		c.JSON(e.ErrorMessage(e.ErrGithubWebHook.AddDesc(errs.Error())))
		c.Abort()
		return
	}

	c.JSON(200, gin.H{"message": output})
}
