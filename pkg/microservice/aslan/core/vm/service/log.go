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

package service

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	utilconfig "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	vmmodel "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/vm"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/util"
)

var VMJobLog = VMJobLogManager{}

type VMJobLogManager struct {
}

func (v *VMJobLogManager) vmJobKey(key string) string {
	return fmt.Sprintf("vm-job-%s", key)
}

func (v *VMJobLogManager) vmJobLogKey(key string) string {
	return fmt.Sprintf("vm-job-log-%s", key)
}

// compressBytes 使用 gzip 压缩字符串，返回压缩后的字节数组
func compressBytes(data string) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write([]byte(data))
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("failed to write data to gzip writer: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}
	return buf.Bytes(), nil
}

// decompressBytes 从压缩的字节数组中解压缩数据
func decompressBytes(compressedData []byte) (string, error) {
	reader, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, reader)
	if err != nil {
		return "", fmt.Errorf("failed to decompress data: %w", err)
	}

	return buf.String(), nil
}

func (v *VMJobLogManager) IsJobRunning(key string) bool {
	exists, err := cache.NewRedisCache(utilconfig.RedisCommonCacheTokenDB()).Exists(v.vmJobKey(key))
	if err != nil {
		log.Errorf("redis check err: %s for key: %s", err, v.vmJobKey(key))
	}
	return exists
}

func (v *VMJobLogManager) SetJobStatusRunning(key string) {
	// use the timeout value of task timeout should be better
	err := cache.NewRedisCache(utilconfig.RedisCommonCacheTokenDB()).SetNX(v.vmJobKey(key), "1", 24*time.Hour)
	if err != nil {
		log.Errorf("reids set nx err: %s for key: %s", err, v.vmJobKey(key))
	}
}

func (v *VMJobLogManager) FinishJob(key string) {
	cache.NewRedisCache(utilconfig.RedisCommonCacheTokenDB()).Delete(v.vmJobKey(key))
}

func (v *VMJobLogManager) GetJobLog(key string) (string, error) {
	redisCache := cache.NewRedisCache(utilconfig.RedisCommonCacheTokenDB())
	logKey := v.vmJobLogKey(key)

	compressedDataStr, err := redisCache.GetString(logKey)
	if err != nil {
		return "", err
	}

	decompressedData, err := decompressBytes([]byte(compressedDataStr))
	if err != nil {
		return "", fmt.Errorf("failed to decompress job log, key: %s, error: %s", logKey, err)
	}

	return decompressedData, nil
}

func (v *VMJobLogManager) WriteJobLog(key string, logContent string) error {
	redisCache := cache.NewRedisCache(utilconfig.RedisCommonCacheTokenDB())
	logKey := v.vmJobLogKey(key)

	// 读取现有的日志内容
	existingCompressedLogStr, err := redisCache.GetString(logKey)
	if err != nil && !errors.Is(err, redis.Nil) {
		// 如果错误不是 key 不存在，则返回错误
		return fmt.Errorf("failed to read existing vm job log from redis, key: %s, error: %s", logKey, err)
	}

	// 追加新内容到现有日志
	var newLogContent string
	if err == nil && existingCompressedLogStr != "" {
		// 解压缩现有日志（将字符串转换为字节数组）
		existingLog, decompressErr := decompressBytes([]byte(existingCompressedLogStr))
		if decompressErr != nil {
			return fmt.Errorf("failed to decompress existing log, key: %s, error: %s", logKey, decompressErr)
		}
		newLogContent = existingLog + logContent
	} else {
		newLogContent = logContent
	}

	compressedLogBytes, err := compressBytes(newLogContent)
	if err != nil {
		return fmt.Errorf("failed to compress job log, key: %s, error: %s", logKey, err)
	}

	// 写回 Redis
	err = redisCache.Write(logKey, string(compressedLogBytes), 3*24*time.Hour)
	if err != nil {
		err = fmt.Errorf("failed to write vm job log to redis, key: %s, error: %s", logKey, err)
		return err
	}

	return nil
}

func (v *VMJobLogManager) DeleteJobLog(key string) {
	cache.NewRedisCache(utilconfig.RedisCommonCacheTokenDB()).Delete(v.vmJobLogKey(key))
}

func savaVMJobLog(job *vmmodel.VMJob, logContent string, logger *zap.SugaredLogger) (err error) {
	if job.Status == string(config.StatusRunning) {
		VMJobLog.SetJobStatusRunning(job.ID.Hex())
	}

	if job != nil && job.LogFile == "" && logContent != "" {
		job.LogFile = VMJobLog.vmJobLogKey(job.ID.Hex())
	}

	if logContent != "" {
		err = VMJobLog.WriteJobLog(job.ID.Hex(), logContent)
		if err != nil {
			return fmt.Errorf("failed to write log to file, error: %s", err)
		}
	}

	// after the task execution ends, synchronize the logs in the file to s3
	if job.JobFinished() {
		if err = uploadVMJobLog2S3(job); err != nil {
			logger.Errorf("failed to upload job log to s3, project: %s, workflow: %s, taskID: %d, error: %s", job.ProjectName, job.WorkflowName, job.TaskID, err)
			return fmt.Errorf("failed to upload job log to s3, project: %s, workflow: %s, taskID: %d, error: %s", job.ProjectName, job.WorkflowName, job.TaskID, err)
		}

		time.Sleep(1000 * time.Millisecond)
		VMJobLog.FinishJob(job.ID.Hex())
	}
	return
}

func uploadVMJobLog2S3(job *vmmodel.VMJob) error {
	store, err := commonrepo.NewS3StorageColl().FindDefault()
	if err != nil {
		return fmt.Errorf("failed to get default s3 storage: %s", err)
	}

	if job.LogFile != "" {
		if store.Subfolder != "" {
			store.Subfolder = fmt.Sprintf("%s/%s/%d/%s", store.Subfolder, strings.ToLower(job.WorkflowName), job.TaskID, "log")
		} else {
			store.Subfolder = fmt.Sprintf("%s/%d/%s", strings.ToLower(job.WorkflowName), job.TaskID, "log")
		}
		s3client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Region, store.Insecure, store.Provider)
		if err != nil {
			return fmt.Errorf("saveContainerLog s3 create client error: %v", err)
		}

		logContent, err := VMJobLog.GetJobLog(job.ID.Hex())
		if err != nil {
			return fmt.Errorf("failed to get vm job log from redis, project: %s, workflow: %s, taskID: %d error: %s", job.ProjectName, job.WorkflowName, job.TaskID, err)
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

		fileName := strings.Replace(strings.ToLower(job.JobName), "_", "-", -1)
		objectKey := GetObjectPath(store.Subfolder, fileName+".log")
		if err = s3client.Upload(
			store.Bucket,
			tempFile,
			objectKey,
		); err != nil {
			return fmt.Errorf("saveContainerLog s3 Upload error: %v", err)
		}

		util.Go(func() {
			time.Sleep(5 * time.Second)
			VMJobLog.DeleteJobLog(job.ID.Hex())
		})

		log.Infof("saveContainerLog s3 upload success, workflowName:%s jobName:%s, taskID:%d", job.WorkflowName, job.JobName, job.TaskID)
	}

	return nil
}

func GetObjectPath(subFolder, name string) string {
	// target should not be started with /
	if subFolder != "" {
		return strings.TrimLeft(filepath.Join(subFolder, name), "/")
	}

	return strings.TrimLeft(name, "/")
}
