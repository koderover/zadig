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
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util"
)

const (
	PERPAGE = 100
	PAGE    = 1
)

func GetTestLocalTestSuite(serviceName string, log *zap.SugaredLogger) (*commonmodels.TestSuite, error) {
	var (
		i            = 1
		pipelineName = fmt.Sprintf("%s-%s", serviceName, "job")
		testReport   = new(commonmodels.TestSuite)
	)
	for {
		pipelineTasks, err := commonrepo.NewTaskColl().ListTasks(&commonrepo.ListAllTaskOption{Type: config.TestType, Limit: PERPAGE, Skip: (i - 1) * PERPAGE})
		if err != nil {
			return nil, fmt.Errorf("GetTestLocalTestSuite serviceName:%s ListTasks err:%v", serviceName, err)
		}
		//处理任务
		for _, pipelineTask := range pipelineTasks {
			stageArray := pipelineTask.Stages
			for _, subStage := range stageArray {
				taskType := subStage.TaskType
				switch taskType {
				case config.TaskTestingV2:
					subTestTaskMap := subStage.SubTasks
					for _, subTask := range subTestTaskMap {
						testInfo, err := base.ToTestingTask(subTask)
						if err != nil {
							continue
						}
						if testInfo.TestModuleName == serviceName {
							testJobName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
								config.TestType, pipelineName, pipelineTask.TaskID, config.TaskTestingV2, serviceName)), "_", "-", -1)

							var storage *s3.S3
							var filename string
							if storage, err = s3.FindDefaultS3(); err == nil {
								filename, err = util.GenerateTmpFile()
								defer func() {
									_ = os.Remove(filename)
								}()
								if err != nil {
									log.Errorf("GetTestLocalTestSuite GenerateTmpFile err:%v", err)
								}
								forcedPathStyle := false
								if storage.Provider == setting.ProviderSourceSystemDefault {
									forcedPathStyle = true
								}
								client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Insecure, forcedPathStyle)
								if err != nil {
									log.Errorf("GetTestLocalTestSuite Create S3 client err:%+v", err)
									continue
								}
								objectKey := storage.GetObjectPath(fmt.Sprintf("%s/%d/%s/%s", pipelineName, pipelineTask.TaskID, "test", testJobName))
								if err = client.Download(storage.Bucket, objectKey, filename); err != nil {
									continue
								}
							} else {
								log.Errorf("GetTestLocalTestSuite FindDefaultS3 err:%v", err)
								continue
							}
							b, err := ioutil.ReadFile(filename)
							if err != nil {
								msg := fmt.Sprintf("GetTestLocalTestSuite get local test result file error: %v", err)
								log.Error(msg)
								continue
							}

							if err := xml.Unmarshal(b, &testReport); err != nil {
								msg := fmt.Sprintf("GetTestLocalTestSuite unmarshal it report xml error: %v", err)
								log.Error(msg)
								continue
							}
							return testReport, nil
						}
					}
				}
			}
		}

		if len(pipelineTasks) < PERPAGE {
			break
		}
		i++
	}

	return testReport, nil
}
