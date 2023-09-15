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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	vmmodel "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/vm"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util"
)

var VMJobStatus = VMJobStatusMap{
	StatusMap: make(map[string]struct{}),
}

type VMJobStatusMap struct {
	StatusMap map[string]struct{}
}

func (v *VMJobStatusMap) Exist(key string) bool {
	_, ok := v.StatusMap[key]
	return ok
}

func (v *VMJobStatusMap) Set(key string) {
	v.StatusMap[key] = struct{}{}
}

func (v *VMJobStatusMap) Delete(key string) {
	time.Sleep(2 * time.Second)
	delete(v.StatusMap, key)
}

func savaVMJobLog(job *vmmodel.VMJob, log string, logger *zap.SugaredLogger) (err error) {
	var file string
	if job != nil && job.LogFile == "" && log != "" {
		file, err = util.CreateFileInCurrentDir(job.ID.Hex())
		if err != nil {
			return fmt.Errorf("failed to generate tmp file, error: %s", err)
		}
		job.LogFile = file
	} else {
		file = job.LogFile
	}

	if log != "" {
		err = util.WriteFile(file, []byte(log), 0644)
		if err != nil {
			return fmt.Errorf("failed to write log to file, error: %s", err)
		}

		if !VMJobStatus.Exist(job.ID.Hex()) {
			VMJobStatus.Set(job.ID.Hex())
		}
	}

	// after the task execution ends, synchronize the logs in the file to s3
	if job.Status == string(config.StatusCancelled) || job.Status == string(config.StatusTimeout) || job.Status == string(config.StatusFailed) || job.Status == string(config.StatusPassed) {
		if err = uploadVMJobLog2S3(job); err != nil {
			logger.Errorf("failed to upload job log to s3, project:%s workflow:%s taskID%d error: %s", job.ProjectName, job.WorkflowName, job.TaskID, err)
			return fmt.Errorf("failed to upload job log to s3, project:%s workflow:%s taskID%d error: %s", job.ProjectName, job.WorkflowName, job.TaskID, err)
		}
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
		forcedPathStyle := true
		if store.Provider == setting.ProviderSourceAli {
			forcedPathStyle = false
		}
		s3client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Region, store.Insecure, forcedPathStyle)
		if err != nil {
			return fmt.Errorf("saveContainerLog s3 create client error: %v", err)
		}
		fileName := strings.Replace(strings.ToLower(job.JobName), "_", "-", -1)
		objectKey := GetObjectPath(store.Subfolder, fileName+".log")
		if err = s3client.Upload(
			store.Bucket,
			job.LogFile,
			objectKey,
		); err != nil {
			return fmt.Errorf("saveContainerLog s3 Upload error: %v", err)
		}

		// remove the log file later
		util.Go(func() {
			VMJobStatus.Delete(job.ID.Hex())
			err = os.Remove(job.LogFile)
			if err != nil {
				log.Errorf("Failed to remove vm job log file, error: %v", err)
			}
		})

		log.Infof("saveContainerLog s3 upload success, workflowName:%s jobName:%s, taskID:%d", job.WorkflowName, job.JobName, job.TaskID)
	} else {
		return fmt.Errorf("saveContainerLog saveFile error: %v", err)
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
