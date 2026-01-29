/*
Copyright 2026 The KodeRover Authors.

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

package joblog

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	pkgconfig "github.com/koderover/zadig/v2/pkg/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/util"
)

type JobLogContext struct {
	WorkflowCtx *commonmodels.WorkflowTaskCtx
	JobTask     *commonmodels.JobTask
}

type JobLogManager struct {
	ctx *JobLogContext
}

// ctx 为nil时，表示不保存log
func NewJobLogManager(ctx *JobLogContext) *JobLogManager {
	return &JobLogManager{ctx: ctx}
}

func (m *JobLogManager) GetJobKey() string {
	if m.ctx == nil {
		log.Errorf("job log context is nil")
		return ""
	}
	return fmt.Sprintf("job-%s-%s-%d-%s", m.ctx.WorkflowCtx.ProjectName, m.ctx.WorkflowCtx.WorkflowName, m.ctx.WorkflowCtx.TaskID, m.ctx.JobTask.Name)
}

func (m *JobLogManager) GetJobKeyWithParams(projectName, workflowName string, taskID int64, jobTaskName string) string {
	return fmt.Sprintf("job-%s-%s-%d-%s", projectName, workflowName, taskID, jobTaskName)
}

func (m *JobLogManager) GetJobLogKey() string {
	if m.ctx == nil {
		log.Errorf("job log context is nil")
		return ""
	}
	return fmt.Sprintf("job-log-%s-%s-%d-%s", m.ctx.WorkflowCtx.ProjectName, m.ctx.WorkflowCtx.WorkflowName, m.ctx.WorkflowCtx.TaskID, m.ctx.JobTask.Name)
}

func (m *JobLogManager) GetJobLogKeyWithParams(projectName, workflowName string, taskID int64, jobTaskName string) string {
	return fmt.Sprintf("job-log-%s-%s-%d-%s", projectName, workflowName, taskID, jobTaskName)
}

func (m *JobLogManager) SaveJobLog(logContent string) (err error) {
	if m.ctx == nil {
		return nil
	}
	logKey := m.GetJobLogKey()

	if logContent != "" {
		logContent = fmt.Sprintf("%s   %s\n", time.Now().Format(setting.WorkflowTimeFormat), logContent)
		err = cache.AppendBigStringToRedis(logKey, logContent)
		if err != nil {
			err = fmt.Errorf("failed to write job log to redis, project: %s, workflow: %s, taskID: %d, error: %s", m.ctx.WorkflowCtx.ProjectName, m.ctx.WorkflowCtx.WorkflowName, m.ctx.WorkflowCtx.TaskID, err)
			log.Errorf(err.Error())
			return err
		}
	}

	return
}

func (m *JobLogManager) SaveJobLogNoTime(logContent string) (err error) {
	if m.ctx == nil {
		log.Errorf("job log context is nil")
		return nil
	}
	logKey := m.GetJobLogKey()

	if logContent != "" {
		logContent = fmt.Sprintf("%s\n", logContent)
		err = cache.AppendBigStringToRedis(logKey, logContent)
		if err != nil {
			err = fmt.Errorf("failed to write job log to redis, project: %s, workflow: %s, taskID: %d, error: %s", m.ctx.WorkflowCtx.ProjectName, m.ctx.WorkflowCtx.WorkflowName, m.ctx.WorkflowCtx.TaskID, err)
			log.Errorf(err.Error())
			return err
		}
	}

	return
}

func (m *JobLogManager) uploadJobLogToS3() error {
	if m.ctx == nil {
		log.Errorf("job log context is nil")
		return fmt.Errorf("job log context is nil")
	}
	store, err := commonrepo.NewS3StorageColl().FindDefault()
	if err != nil {
		return fmt.Errorf("failed to get default s3 storage: %s", err)
	}

	if store.Subfolder != "" {
		store.Subfolder = fmt.Sprintf("%s/%s/%d/%s", store.Subfolder, strings.ToLower(m.ctx.WorkflowCtx.WorkflowName), m.ctx.WorkflowCtx.TaskID, "log")
	} else {
		store.Subfolder = fmt.Sprintf("%s/%d/%s", strings.ToLower(m.ctx.WorkflowCtx.WorkflowName), m.ctx.WorkflowCtx.TaskID, "log")
	}
	s3client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Region, store.Insecure, store.Provider)
	if err != nil {
		return fmt.Errorf("saveContainerLog s3 create client error: %v", err)
	}

	logKey := m.GetJobLogKey()
	logContent, err := cache.GetBigStringFromRedis(logKey)
	if err != nil {
		return fmt.Errorf("failed to get job log from redis, project: %s, workflow: %s, taskID: %d error: %s", m.ctx.WorkflowCtx.ProjectName, m.ctx.WorkflowCtx.WorkflowName, m.ctx.WorkflowCtx.TaskID, err)
	}

	tempFile, err := util.GenerateTmpFile()
	if err != nil {
		return fmt.Errorf("failed to generate tmp file, error: %s", err)
	}
	err = util.WriteFile(tempFile, []byte(logContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write log to file, error: %s", err)
	}

	defer func() {
		_ = os.Remove(tempFile)
	}()

	fileName := strings.Replace(strings.ToLower(m.ctx.JobTask.Name), "_", "-", -1)
	objectKey := getObjectPath(store.Subfolder, fileName+".log")
	if err = s3client.Upload(
		store.Bucket,
		tempFile,
		objectKey,
	); err != nil {
		return fmt.Errorf("saveContainerLog s3 Upload error: %v", err)
	}

	util.Go(func() {
		time.Sleep(5 * time.Second)
		cache.DeleteBigStringFromRedis(logKey)
	})

	log.Infof("uploadJobLogToS3 s3 upload success, workflowName:%s jobTaskKey:%s, taskID:%d", m.ctx.WorkflowCtx.WorkflowName, m.ctx.JobTask.Name, m.ctx.WorkflowCtx.TaskID)

	return nil
}

func (m *JobLogManager) IsJobRunning(key string) bool {
	exists, err := cache.NewRedisCache(pkgconfig.RedisCommonCacheTokenDB()).Exists(key)
	if err != nil {
		log.Errorf("redis check err: %s for key: %s", err, key)
	}
	return exists
}

func (m *JobLogManager) SetJobStatusRunning(key string, timeout time.Duration) {
	err := cache.NewRedisCache(pkgconfig.RedisCommonCacheTokenDB()).SetNX(key, "1", timeout+2*time.Minute)
	if err != nil {
		log.Errorf("redis setNX err: %s for key: %s", err, key)
	}
}

func (m *JobLogManager) FinishJob() {
	if m.ctx == nil {
		log.Errorf("job log context is nil")
		return
	}

	jobKey := m.GetJobKey()
	cache.NewRedisCache(pkgconfig.RedisCommonCacheTokenDB()).Delete(jobKey)

	jobLogKey := m.GetJobLogKey()

	// after the task execution ends, synchronize the logs in the file to s3
	err := m.uploadJobLogToS3()
	if err != nil {
		err = fmt.Errorf("failed to upload job log to s3, project: %s, workflow: %s, taskID: %d, error: %s", m.ctx.WorkflowCtx.ProjectName, m.ctx.WorkflowCtx.WorkflowName, m.ctx.WorkflowCtx.TaskID, err)
		log.Errorf(err.Error())
	}

	time.Sleep(1000 * time.Millisecond)
	cache.DeleteBigStringFromRedis(jobLogKey)

	cache.NewRedisCache(pkgconfig.RedisCommonCacheTokenDB()).Delete(jobLogKey)
}

func (m *JobLogManager) ReadLogFromOffset(logKey string, readOffset int) (*bufio.Reader, int, error) {
	logContent, err := cache.GetBigStringFromRedis(logKey)
	if err != nil {
		log.Errorf("get job log error: %v", err)
		return nil, 0, err
	}
	newLogContent := logContent
	if readOffset <= len(logContent) {
		newLogContent = logContent[readOffset:]
	}

	readOffset = len(logContent)
	buf := bufio.NewReader(strings.NewReader(newLogContent))
	return buf, readOffset, nil
}

func getObjectPath(subFolder, name string) string {
	// target should not be started with /
	if subFolder != "" {
		return strings.TrimLeft(filepath.Join(subFolder, name), "/")
	}

	return strings.TrimLeft(name, "/")
}
