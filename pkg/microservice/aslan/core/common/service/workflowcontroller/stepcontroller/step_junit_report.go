/*
Copyright 2022 The KodeRover Authors.

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

package stepcontroller

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/types/step"
	"github.com/koderover/zadig/pkg/util"
)

type junitReportCtl struct {
	step            *commonmodels.StepTask
	junitReportSpec *step.StepJunitReportSpec
	log             *zap.SugaredLogger
}

func NewJunitReportCtl(stepTask *commonmodels.StepTask, log *zap.SugaredLogger) (*junitReportCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal git spec error: %v", err)
	}
	junitReportSpec := &step.StepJunitReportSpec{}
	if err := yaml.Unmarshal(yamlString, &junitReportSpec); err != nil {
		return nil, fmt.Errorf("unmarshal git spec error: %v", err)
	}
	stepTask.Spec = junitReportSpec
	return &junitReportCtl{junitReportSpec: junitReportSpec, log: log, step: stepTask}, nil
}

func (s *junitReportCtl) PreRun(ctx context.Context) error {
	if s.junitReportSpec.S3Storage == nil {
		modelS3, err := commonrepo.NewS3StorageColl().FindDefault()
		if err != nil {
			return err
		}
		s.junitReportSpec.S3Storage = modelS3toS3(modelS3)
	}
	s.step.Spec = s.junitReportSpec
	return nil
}

func (s *junitReportCtl) AfterRun(ctx context.Context) error {
	if s.junitReportSpec.TestName == "" {
		return nil
	}
	var testTaskStat *commonmodels.TestTaskStat
	var isNew bool
	testTaskStat, _ = commonrepo.NewTestTaskStatColl().FindTestTaskStat(&commonrepo.TestTaskStatOption{Name: s.junitReportSpec.TestName})
	if testTaskStat == nil {
		isNew = true
		testTaskStat = new(commonmodels.TestTaskStat)
		testTaskStat.Name = s.junitReportSpec.TestName
		testTaskStat.CreateTime = time.Now().Unix()
		testTaskStat.UpdateTime = time.Now().Unix()
	}
	filename, err := util.GenerateTmpFile()
	if err != nil {
		log.Errorf("GenerateTmpFile err:%v", err)
		return err
	}
	storage, err := s3.FindDefaultS3()
	if err != nil {
		log.Errorf("find defalt s3 error: %v", err)
		return err
	}
	forcedPathStyle := true
	if storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Insecure, forcedPathStyle)
	if err != nil {
		log.Errorf("NewClient err:%v", err)
		return err
	}
	objectKey := filepath.Join(s.junitReportSpec.S3DestDir, s.junitReportSpec.FileName)
	err = client.Download(storage.Bucket, objectKey, filename)
	if err != nil {
		log.Errorf("Download junit report err:%v", err)
		return err
	}
	defer os.Remove(filename)

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Error("get local test result file error: %v", err)
		return err
	}
	testReport := new(commonmodels.TestSuite)
	if err := xml.Unmarshal(b, &testReport); err != nil {
		log.Error("uploadTaskData testSuite unmarshal it report xml error: %v", err)
		return err
	}
	totalCaseNum := testReport.Tests
	if totalCaseNum != 0 {
		testTaskStat.TestCaseNum = totalCaseNum
	}
	testTaskStat.TotalSuccess++
	if isNew {
		_ = commonrepo.NewTestTaskStatColl().Create(testTaskStat)
	} else {
		testTaskStat.UpdateTime = time.Now().Unix()
		_ = commonrepo.NewTestTaskStatColl().Update(testTaskStat)
	}
	return nil
}
