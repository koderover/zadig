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

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/workflow/service/webhook"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

// Logger is get logger func
func Logger(c *gin.Context) *xlog.Logger {
	logger := xlog.DefaultLogger(c)
	logger.SetLoggerLevel(config.LogLevel())
	return logger
}

func ProcessGithub(c *gin.Context) {
	log := Logger(c)
	errs := &multierror.Error{}

	buf, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(e.ErrorMessage(e.ErrGithubWebHook.AddDesc(errs.Error())))
		c.Abort()
		return
	}

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
	// trigger classic pipeline
	output, err := webhook.ProcessGithubHook(c.Request, log)
	if err != nil {
		log.Errorf("error happens to trigger classic pipeline %v", err)
		errs = multierror.Append(errs, err)
	}

	// trigger workflow
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
	err = webhook.ProcessGithubWebHook(c.Request, log)

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
