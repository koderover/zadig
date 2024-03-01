/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package jobcontroller

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

type SQLJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskSQLSpec
	ack         func()
	dbInfo      *commonmodels.DBInstance
}

func NewSQLJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *SQLJobCtl {
	jobTaskSpec := &commonmodels.JobTaskSQLSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &SQLJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *SQLJobCtl) Clean(ctx context.Context) {}

func (c *SQLJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	info, err := mongodb.NewDBInstanceColl().Find(&mongodb.DBInstanceCollFindOption{Id: c.jobTaskSpec.ID})
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}
	c.dbInfo = info

	switch info.Type {
	case config.DBInstanceTypeMySQL, config.DBInstanceTypeMariaDB:
		if err := c.ExecMySQLStatement(); err != nil {
			logError(c.job, err.Error(), c.logger)
			return
		}
	default:
		logError(c.job, "invalid db type", c.logger)
		return
	}

	c.job.Status = config.StatusPassed
	return
}

func (c *SQLJobCtl) ExecMySQLStatement() error {
	info := c.dbInfo

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8&multiStatements=true", info.Username, info.Password, info.Host, info.Port))
	if err != nil {
		return errors.Errorf("connect db error: %v", err)
	}
	defer db.Close()

	// 插入示例
	_, err = db.Exec(c.jobTaskSpec.SQL)
	if err != nil {
		return errors.Errorf("exec SQL error: %v", err)
	}
	return nil
}

func (c *SQLJobCtl) SaveInfo(ctx context.Context) error {
	return mongodb.NewJobInfoColl().Create(context.TODO(), &commonmodels.JobInfo{
		Type:                c.job.JobType,
		WorkflowName:        c.workflowCtx.WorkflowName,
		WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID:              c.workflowCtx.TaskID,
		ProductName:         c.workflowCtx.ProjectName,
		StartTime:           c.job.StartTime,
		EndTime:             c.job.EndTime,
		Duration:            c.job.EndTime - c.job.StartTime,
		Status:              string(c.job.Status),
	})
}
