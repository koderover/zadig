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
	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	cronservice "github.com/koderover/zadig/lib/microservice/aslan/core/cron/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
)

func CleanJobCronJob(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	cronservice.CleanJobCronJob(ctx.Logger)
}

func CleanConfigmapCronJob(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	cronservice.CleanConfigmapCronJob(ctx.Logger)
}

// param type: cronjob的执行内容类型
// param name: 当type为workflow的时候 代表workflow名称， 当type为test的时候，为test名称
type DisableCronjobReq struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func DisableCronjob(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(DisableCronjobReq)

	if err := c.BindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = cronservice.DisableCronjob(args.Name, args.Type, ctx.Logger)
}

type cronjobResp struct {
	ID           string                         `json:"_id,omitempty"`
	Name         string                         `json:"name"`
	Type         string                         `json:"type"`
	Number       uint64                         `json:"number"`
	Frequency    string                         `json:"frequency"`
	Time         string                         `json:"time"`
	Cron         string                         `json:"cron"`
	ProductName  string                         `json:"product_name,omitempty"`
	MaxFailure   int                            `json:"max_failures,omitempty"`
	TaskArgs     *commonmodels.TaskArgs         `json:"task_args,omitempty"`
	WorkflowArgs *commonmodels.WorkflowTaskArgs `json:"workflow_args,omitempty"`
	TestArgs     *commonmodels.TestTaskArgs     `json:"test_args,omitempty"`
	JobType      string                         `json:"job_type"`
	Enabled      bool                           `json:"enabled"`
}

func ListActiveCronjobFailsafe(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	var resp []*cronjobResp

	cronjobList, err := cronservice.ListActiveCronjobFailsafe()
	for _, cronjob := range cronjobList {
		resp = append(resp, &cronjobResp{
			ID:           cronjob.ID.Hex(),
			Name:         cronjob.Name,
			Type:         cronjob.Type,
			Number:       cronjob.Number,
			Frequency:    cronjob.Frequency,
			Time:         cronjob.Time,
			Cron:         cronjob.Cron,
			ProductName:  cronjob.ProductName,
			MaxFailure:   cronjob.MaxFailure,
			TaskArgs:     cronjob.TaskArgs,
			WorkflowArgs: cronjob.WorkflowArgs,
			TestArgs:     cronjob.TestArgs,
			JobType:      cronjob.JobType,
			Enabled:      cronjob.Enabled,
		})
	}
	ctx.Resp = resp
	ctx.Err = err
}

func ListActiveCronjob(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	var resp []*cronjobResp

	cronjobList, err := cronservice.ListActiveCronjob()
	for _, cronjob := range cronjobList {
		resp = append(resp, &cronjobResp{
			ID:           cronjob.ID.Hex(),
			Name:         cronjob.Name,
			Type:         cronjob.Type,
			Number:       cronjob.Number,
			Frequency:    cronjob.Frequency,
			Time:         cronjob.Time,
			Cron:         cronjob.Cron,
			ProductName:  cronjob.ProductName,
			MaxFailure:   cronjob.MaxFailure,
			TaskArgs:     cronjob.TaskArgs,
			WorkflowArgs: cronjob.WorkflowArgs,
			TestArgs:     cronjob.TestArgs,
			JobType:      cronjob.JobType,
			Enabled:      cronjob.Enabled,
		})
	}
	ctx.Resp = resp
	ctx.Err = err
}

func ListCronjob(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	var resp []*cronjobResp

	name := c.Param("name")
	pType := c.Param("type")

	cronjobList, err := cronservice.ListCronjob(name, pType)
	for _, cronjob := range cronjobList {
		resp = append(resp, &cronjobResp{
			ID:           cronjob.ID.Hex(),
			Name:         cronjob.Name,
			Type:         cronjob.Type,
			Number:       cronjob.Number,
			Frequency:    cronjob.Frequency,
			Time:         cronjob.Time,
			Cron:         cronjob.Cron,
			ProductName:  cronjob.ProductName,
			MaxFailure:   cronjob.MaxFailure,
			TaskArgs:     cronjob.TaskArgs,
			WorkflowArgs: cronjob.WorkflowArgs,
			TestArgs:     cronjob.TestArgs,
			JobType:      cronjob.JobType,
			Enabled:      cronjob.Enabled,
		})
	}
	ctx.Resp = resp
	ctx.Err = err
}
