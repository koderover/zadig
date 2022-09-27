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

package service

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/types/step"
	"github.com/koderover/zadig/pkg/util/fs"
)

type GetTestArtifactInfoResp struct {
	FileNames          []string `json:"file_names"`
	NotHistoryFileFlag bool     `json:"not_history_file_flag"`
}

func GetTestArtifactInfo(pipelineName, dir string, taskID int64, log *zap.SugaredLogger) (*GetTestArtifactInfoResp, error) {
	resp := new(GetTestArtifactInfoResp)
	fis := make([]string, 0)

	storage, err := s3.FindDefaultS3()
	if err != nil {
		log.Errorf("GetTestArtifactInfo FindDefaultS3 err:%v", err)
		return resp, err
	}
	if storage.Subfolder != "" {
		storage.Subfolder = fmt.Sprintf("%s/%s/%d/%s", storage.Subfolder, pipelineName, taskID, "artifact")
	} else {
		storage.Subfolder = fmt.Sprintf("%s/%d/%s", pipelineName, taskID, "artifact")
	}
	forcedPathStyle := true
	if storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Insecure, forcedPathStyle)
	if err != nil {
		log.Errorf("GetTestArtifactInfo create s3 client err:%v", err)
		return resp, err
	}

	objectKey := storage.GetObjectPath(fmt.Sprintf("%s/%s/%s", dir, "workspace", setting.ArtifactResultOut))
	object, err := client.GetFile(storage.Bucket, objectKey, &s3tool.DownloadOption{IgnoreNotExistError: true, RetryNum: 2})
	if err != nil {
		log.Errorf("GetTestArtifactInfo GetFile err:%s", err)
		return resp, err
	}
	if object != nil && *object.ContentLength > 0 {
		tempDir, _ := ioutil.TempDir("", "")
		sourcePath := path.Join(tempDir, "artifact")
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			_ = os.MkdirAll(sourcePath, 0777)
		}

		file, err := os.Create(path.Join(sourcePath, setting.ArtifactResultOut))
		if err != nil {
			return nil, fmt.Errorf("failed to create file %s %s", setting.ArtifactResultOut, err)
		}
		defer func() {
			_ = file.Close()
			_ = os.Remove(tempDir)
		}()

		err = fs.SaveFile(object.Body, file.Name())
		if err != nil {
			log.Errorf("Failed to save file to %s, err: %s", file.Name(), err)
			return resp, err
		}

		cmdtf := exec.Command("tar", "-tf", file.Name())
		var stdoutBuf bytes.Buffer
		cmdtf.Stdout = &stdoutBuf
		cmdtf.Stderr = os.Stderr
		if err = cmdtf.Run(); err != nil {
			fmt.Printf("failed to tar -tf err:%s", err)
			return resp, err
		}
		for _, tarFile := range strings.Split(string(stdoutBuf.Bytes()), "\n") {
			if strings.HasSuffix(tarFile, "/") {
				continue
			}
			if strings.TrimSpace(tarFile) == "" {
				continue
			}
			fis = append(fis, tarFile)
		}
		resp.FileNames = fis
		resp.NotHistoryFileFlag = true
		return resp, nil
	}

	prefix := storage.GetObjectPath(dir)
	files, err := client.ListFiles(storage.Bucket, prefix, true)
	if err != nil || len(files) <= 0 {
		log.Errorf("GetTestArtifactInfo ListFiles err:%s", err)
		return resp, err
	}

	for _, file := range files {
		_, fileName := path.Split(file)
		fis = append(fis, fileName)
	}
	resp.FileNames = fis
	return resp, nil
}

func GetWorkflowV4TestArtifactInfo(workflowName, jobName string, taskID int64, log *zap.SugaredLogger) (*GetTestArtifactInfoResp, error) {
	resp := new(GetTestArtifactInfoResp)
	workflowTask, err := mongodb.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		return resp, fmt.Errorf("cannot find workflow task, workflow name: %s, task id: %d", workflowName, taskID)
	}
	var jobTask *commonmodels.JobTask
	for _, stage := range workflowTask.Stages {
		for _, job := range stage.Jobs {
			if job.Name != jobName {
				continue
			}
			if job.JobType != string(config.JobZadigTesting) {
				return resp, fmt.Errorf("job: %s was not a testing job", jobName)
			}
			jobTask = job
		}
	}
	if jobTask == nil {
		return resp, fmt.Errorf("cannot find job task, workflow name: %s, task id: %d, job name: %s", workflowName, taskID, jobName)
	}
	jobSpec := &commonmodels.JobTaskFreestyleSpec{}
	if err := commonmodels.IToi(jobTask.Spec, jobSpec); err != nil {
		return resp, fmt.Errorf("unmashal job spec error: %v", err)
	}

	var stepTask *commonmodels.StepTask
	for _, step := range jobSpec.Steps {
		if step.Name != config.TestJobArchiveResultStepName {
			continue
		}
		if step.StepType != config.StepTarArchive {
			return resp, fmt.Errorf("step: %s was not a junit report step", step.Name)
		}
		stepTask = step
	}
	if stepTask == nil {
		return resp, fmt.Errorf("cannot find step task, workflow name: %s, task id: %d, job name: %s", workflowName, taskID, jobName)
	}
	stepSpec := &step.StepTarArchiveSpec{}
	if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil {
		return resp, fmt.Errorf("unmashal step spec error: %v", err)
	}

	fis := make([]string, 0)

	storage, err := s3.FindDefaultS3()
	if err != nil {
		log.Errorf("GetTestArtifactInfo FindDefaultS3 err:%v", err)
		return resp, err
	}
	forcedPathStyle := true
	if storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Insecure, forcedPathStyle)
	if err != nil {
		log.Errorf("GetTestArtifactInfo create s3 client err:%v", err)
		return resp, err
	}

	objectKey := filepath.Join(stepSpec.S3DestDir, stepSpec.FileName)
	object, err := client.GetFile(storage.Bucket, objectKey, &s3tool.DownloadOption{RetryNum: 2})
	if err != nil {
		log.Errorf("GetTestArtifactInfo GetFile err:%s", err)
		return resp, err
	}
	if object != nil && *object.ContentLength > 0 {
		tempDir, _ := ioutil.TempDir("", "")
		sourcePath := path.Join(tempDir, "artifact")
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			_ = os.MkdirAll(sourcePath, 0777)
		}

		file, err := os.Create(path.Join(sourcePath, setting.ArtifactResultOut))
		if err != nil {
			return nil, fmt.Errorf("failed to create file %s %s", setting.ArtifactResultOut, err)
		}
		defer func() {
			_ = file.Close()
			_ = os.Remove(tempDir)
		}()

		err = fs.SaveFile(object.Body, file.Name())
		if err != nil {
			log.Errorf("Failed to save file to %s, err: %s", file.Name(), err)
			return resp, err
		}

		cmdtf := exec.Command("tar", "-tf", file.Name())
		var stdoutBuf bytes.Buffer
		cmdtf.Stdout = &stdoutBuf
		cmdtf.Stderr = os.Stderr
		if err = cmdtf.Run(); err != nil {
			fmt.Printf("failed to tar -tf err:%s", err)
			return resp, err
		}
		for _, tarFile := range strings.Split(string(stdoutBuf.Bytes()), "\n") {
			if strings.HasSuffix(tarFile, "/") {
				continue
			}
			if strings.TrimSpace(tarFile) == "" {
				continue
			}
			fis = append(fis, tarFile)
		}
		resp.FileNames = fis
		resp.NotHistoryFileFlag = true
		return resp, nil
	}

	prefix := storage.GetObjectPath("")
	files, err := client.ListFiles(storage.Bucket, prefix, true)
	if err != nil || len(files) <= 0 {
		log.Errorf("GetTestArtifactInfo ListFiles err:%s", err)
		return resp, err
	}

	for _, file := range files {
		_, fileName := path.Split(file)
		fis = append(fis, fileName)
	}
	resp.FileNames = fis
	return resp, nil
}
