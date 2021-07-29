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

package taskplugin

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/taskplugin/s3"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
)

// InitializeDistribute2S3TaskPlugin ...
func InitializeDistribute2S3TaskPlugin(taskType config.TaskType) TaskPlugin {
	return &Distribute2S3TaskPlugin{
		Name: taskType,
	}
}

// Distribute2S3TaskPlugin Plugin name should be compatible with task type
type Distribute2S3TaskPlugin struct {
	Name config.TaskType
	Task *task.DistributeToS3
	Log  *zap.SugaredLogger
}

func (p *Distribute2S3TaskPlugin) SetAckFunc(func()) {
}

const (
	// Distribute2S3TaskTimeout ...
	Distribute2S3TaskTimeout = 60 * 10 // 10 minutes
)

// Init ...
func (p *Distribute2S3TaskPlugin) Init(jobname, filename string, xl *zap.SugaredLogger) {
	// SetLogger ...
	p.Log = xl
}

// Type ...
func (p *Distribute2S3TaskPlugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *Distribute2S3TaskPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *Distribute2S3TaskPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
func (p *Distribute2S3TaskPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = Distribute2S3TaskTimeout
	}
	return p.Task.Timeout
}

//是否是kodo
func isKODO(storage *s3.S3) bool {
	return strings.Contains(storage.Endpoint, "qiniucs.com")
}

func upload(ctx context.Context, log *zap.SugaredLogger, storage *s3.S3, localfile, destfile string) error {
	if isKODO(storage) {
		err := uploadFileToS3(storage.Ak, storage.Sk, storage.Bucket, destfile, localfile)
		if err != nil {
			log.Warnf("failed to upload file to s3 %s=>%s %v", localfile, destfile, err)
		} else {
			log.Infof("succeed to upload file to s3 %s=>%s", localfile, destfile)
		}

		return err
	}
	forcedPathStyle := false
	if storage.Provider == setting.ProviderSourceSystemDefault {
		forcedPathStyle = true
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Insecure, forcedPathStyle)
	if err != nil {
		return err
	}

	objectKey := storage.GetObjectPath(destfile)
	return client.Upload(storage.Bucket, localfile, objectKey)
}

// Run ...
func (p *Distribute2S3TaskPlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	var err error

	defer func() {
		if err != nil {
			p.Task.TaskStatus = config.StatusFailed
			p.Task.Error = err.Error()
		}
	}()

	localFile := filepath.Join(pipelineCtx.DistDir, p.Task.PackageFile)

	if _, err = os.Stat(localFile); os.IsNotExist(err) {
		if pipelineTask.StorageURI == "" {
			err = fmt.Errorf("no source storage found")
			return
		}

		var srcStorage *s3.S3
		srcStorage, err = s3.NewS3StorageFromEncryptedURI(pipelineTask.StorageURI)
		if err != nil {
			p.Log.Errorf("failed to init source s3 client")
			return
		}
		if srcStorage.Subfolder != "" {
			srcStorage.Subfolder = fmt.Sprintf("%s/%s/%d/%s", srcStorage.Subfolder, pipelineTask.PipelineName, pipelineTask.TaskID, "file")
		} else {
			srcStorage.Subfolder = fmt.Sprintf("%s/%d/%s", pipelineTask.PipelineName, pipelineTask.TaskID, "file")
		}

		var tmpFile *os.File

		tmpFile, err = ioutil.TempFile("", "")
		if err != nil {
			p.Log.Errorf("failed to create tempfile %v", err)
			return
		}

		_ = tmpFile.Close()

		defer func() {
			_ = os.Remove(tmpFile.Name())
		}()
		forcedPathStyle := false
		if srcStorage.Provider == setting.ProviderSourceSystemDefault {
			forcedPathStyle = true
		}
		s3client, err := s3tool.NewClient(srcStorage.Endpoint, srcStorage.Ak, srcStorage.Sk, srcStorage.Insecure, forcedPathStyle)
		if err != nil {
			p.Log.Errorf("failed to create s3 client source storage %s: %v", srcStorage.GetURI(), err)
			return
		}
		objectKey := srcStorage.GetObjectPath(p.Task.PackageFile)
		err = s3client.Download(srcStorage.Bucket, objectKey, tmpFile.Name())
		if err != nil {
			p.Log.Errorf("failed to download file from source storage %s: %v", srcStorage.GetURI(), err)
			return
		}

		localFile = tmpFile.Name()
	}

	var destStorage *s3.S3
	destStorage, err = s3.NewS3StorageFromEncryptedURI(p.Task.DestStorageURL)
	if err != nil {
		p.Log.Errorf("failed to init destination s3 client %v", err)
		return
	}

	p.Log.Infof("upload package file %s to s3\n", p.Task.PackageFile)
	p.Log.Infof("upload file %s to bucket %s/%s", p.Task.PackageFile, destStorage.Bucket, destStorage.Subfolder)

	remoteFileKey := filepath.Join(
		"/", pipelineTask.ProductName,
		p.Task.ServiceName,
		fmt.Sprintf("%d", pipelineTask.TaskID))

	if isKODO(destStorage) {
		remoteFileKey = filepath.Join(remoteFileKey, p.Task.PackageFile)
	}

	if destStorage.Subfolder != "" {
		destStorage.Subfolder = fmt.Sprintf("%s/%s", destStorage.Subfolder, remoteFileKey)
	} else {
		destStorage.Subfolder = remoteFileKey
	}

	remoteFileName := p.Task.PackageFile
	p.Task.RemoteFileKey = filepath.Join(destStorage.Subfolder, p.Task.PackageFile)

	if isKODO(destStorage) {
		remoteFileName = remoteFileKey
		p.Task.RemoteFileKey = destStorage.Subfolder
	}

	err = upload(ctx, p.Log, destStorage, localFile, remoteFileName)
	if err != nil {
		p.Log.Errorf("failed to upload file to dest storage %s %v", destStorage.GetURI(), err)
		return
	}

	p.Log.Infof(
		"md5sum enabled, will do md5sum and upload file %s.md5 to bucket %s",
		p.Task.PackageFile,
		destStorage.Bucket,
	)

	localMd5File := fmt.Sprintf("%s.md5", localFile)

	var f *os.File
	f, err = os.Open(localFile)
	if err != nil {
		p.Log.Errorf("open local file error: %v", err)
		return
	}

	defer func() {
		_ = f.Close()
	}()

	h := md5.New()
	if _, err = io.Copy(h, f); err != nil {
		p.Log.Errorf("copy md5 error: %v", err)
		return
	}

	err = ioutil.WriteFile(localMd5File, []byte(fmt.Sprintf("%x", h.Sum(nil))), 0644)
	if err != nil {
		p.Log.Errorf("write md5 file error: %v", err)
		return
	}

	defer func() {
		_ = os.Remove(localMd5File)
	}()

	remoteMd5File := fmt.Sprintf("%s.md5", p.Task.PackageFile)

	if isKODO(destStorage) {
		remoteMd5File = fmt.Sprintf("%s.md5", remoteFileKey)
	}

	err = upload(ctx, p.Log, destStorage, localMd5File, remoteMd5File)
	if err != nil {
		p.Log.Errorf("failed to upload md5 file to %s %v", destStorage.GetURI(), err)
		return
	}

	p.Task.TaskStatus = config.StatusPassed
}

// Wait ...
func (p *Distribute2S3TaskPlugin) Wait(ctx context.Context) {

	timeout := time.After(time.Duration(p.TaskTimeout()) * time.Second)

	for {
		select {
		case <-ctx.Done():
			p.Task.TaskStatus = config.StatusCancelled
			return

		case <-timeout:
			p.Task.TaskStatus = config.StatusTimeout
			return

		default:
			time.Sleep(time.Second * 1)

			if p.IsTaskDone() {
				return
			}
		}
	}
}

// Complete ...
func (p *Distribute2S3TaskPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
}

// SetTask ...
func (p *Distribute2S3TaskPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToDistributeToS3Task(t)
	if err != nil {
		return err
	}
	p.Task = task
	return nil
}

// GetTask ...
func (p *Distribute2S3TaskPlugin) GetTask() interface{} {
	return p.Task
}

// IsTaskDone ...
func (p *Distribute2S3TaskPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *Distribute2S3TaskPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *Distribute2S3TaskPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *Distribute2S3TaskPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (p *Distribute2S3TaskPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *Distribute2S3TaskPlugin) ResetError() {
	p.Task.Error = ""
}
