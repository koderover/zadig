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

package workflow

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	dockerCli "github.com/docker/docker/client"
	"github.com/docker/go-connections/sockets"
	"github.com/nsqio/go-nsq"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	git "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/notify"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/registry"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/poetry"
	"github.com/koderover/zadig/pkg/tool/git/gitlab"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util"
)

var once sync.Once

type TaskAckHandler struct {
	queue                *Queue
	ptColl               *commonrepo.TaskColl
	productColl          *commonrepo.ProductColl
	pColl                *commonrepo.PipelineColl
	wColl                *commonrepo.WorkflowColl
	deliveryArtifactColl *commonrepo.DeliveryArtifactColl
	deliveryActivityColl *commonrepo.DeliveryActivityColl
	TestTaskStatColl     *commonrepo.TestTaskStatColl
	PoetryClient         *poetry.Client
	messages             chan *nsq.Message
	log                  *zap.SugaredLogger
}

func NewTaskAckHandler(poetryServer, poetryRootKey string, maxInFlight int, log *zap.SugaredLogger) *TaskAckHandler {
	return &TaskAckHandler{
		queue:                NewPipelineQueue(log),
		ptColl:               commonrepo.NewTaskColl(),
		productColl:          commonrepo.NewProductColl(),
		pColl:                commonrepo.NewPipelineColl(),
		wColl:                commonrepo.NewWorkflowColl(),
		deliveryArtifactColl: commonrepo.NewDeliveryArtifactColl(),
		deliveryActivityColl: commonrepo.NewDeliveryActivityColl(),
		TestTaskStatColl:     commonrepo.NewTestTaskStatColl(),
		PoetryClient:         poetry.New(poetryServer, poetryRootKey),
		messages:             make(chan *nsq.Message, maxInFlight*10),
		log:                  log,
	}
}

// HandleMessage 接收 warpdrive 回传 pipeline task 消息
// 1. 更新 queue pipeline task
// 2. 更新 数据库 pipeline task
// 3. 如果 pipeline task 完成, 检查是否有 blocked pipeline task, 检查是否可以unblock, 从queue中移除task
//    - pipeline 完成状态包括：passed, failed, timeout
// 4. 更新 数据库 proudct
// 5. 更新历史piplinetask的状态为archived(默认只留下最近的一百个task)
func (h *TaskAckHandler) HandleMessage(message *nsq.Message) error {
	once.Do(func() {
		go func() {
			for msg := range h.messages {
				_ = h.handle(msg)
			}
		}()
	})

	h.messages <- message
	return nil
}

func (h *TaskAckHandler) handle(message *nsq.Message) error {
	if message == nil {
		h.log.Error("nil nsq.Message")
		return nil
	}

	var pt *task.Task
	if err := json.Unmarshal(message.Body, &pt); err != nil {
		h.log.Errorf("unmarshal PipelineTaskV2 message error: %v", err)
		return nil
	}

	ptInfo := fmt.Sprintf("%s %s-%d(%s)", message.ID, pt.PipelineName, pt.TaskID, pt.Status)
	h.log.Infof("[%s]receive task ACK: %+v", ptInfo, pt)

	start := time.Now()
	defer func() {
		h.log.Infof(
			"[%s] handle ack message takes %s",
			ptInfo,
			time.Since(start).String(),
		)
	}()

	// 从数据库中查询当前任务状态
	taskInColl, err := h.ptColl.Find(pt.TaskID, pt.PipelineName, pt.Type)
	if err != nil {
		h.log.Errorf("find pipeline task %s:%d error: %v", pt.PipelineName, pt.TaskID, err)
		return nil
	}

	// 如果当前状态已经通过或者失败, 不处理新接受到的ACK
	if taskInColl.Status == config.StatusPassed || taskInColl.Status == config.StatusFailed || taskInColl.Status == config.StatusTimeout {
		h.log.Infof("%s:%d:%s task already done", pt.PipelineName, pt.TaskID, taskInColl.Status)
		return nil
	}
	// 如果当前状态已经是取消状态, 一般为用户取消了任务, 此时任务在短暂时间内会继续运行一段时间,
	if taskInColl.Status == config.StatusCancelled {
		// Task终止状态可能为Pass, Fail, Cancel, Timeout
		// backend 会继续接受到ACK, 在这种情况下, 终止状态之外的ACK都无需处理，避免出现取消之后又被重置成运行态
		if pt.Status != config.StatusFailed && pt.Status != config.StatusPassed && pt.Status != config.StatusCancelled && pt.Status != config.StatusTimeout {
			h.log.Infof("%s:%d task has been cancelled, ACK dropped", pt.PipelineName, pt.TaskID)
			return nil
		}
	}

	// 更新队列中任务状态
	h.queue.Update(pt)

	// 更新数据库未完成任务状态
	if err := h.ptColl.UpdateUnfinishedTask(pt); err != nil {
		h.log.Errorf("%s:%d UpdateUnfinishedTask error: %v", pt.PipelineName, pt.TaskID, err)
		return nil
	}

	// 如果任务完成：成功、失败、超时
	if pt.Status == config.StatusPassed || pt.Status == config.StatusFailed || pt.Status == config.StatusTimeout {
		h.log.Infof("%s:%d:%v task done", pt.PipelineName, pt.TaskID, pt.Status)
		h.queue.Remove(pt)
		go func() {
			if err = h.uploadTaskData(pt); err != nil {
				h.log.Errorf("uploadTaskData err: %v", err)
			}
		}()

		go func() {
			if err = h.updateWorkflowStat(pt); err != nil {
				h.log.Errorf("updateWorkflowStat err: %v", err)
			}
		}()

		go func() {
			if err = h.createVersion(pt); err != nil {
				h.log.Errorf("createVersion err: %v", err)
			}
		}()
	}

	// 更新数据库 product
	var deploys []*task.Deploy

	for _, stage := range pt.Stages {
		tasks := make([]map[string]interface{}, 0)
		for _, v := range stage.SubTasks {
			tasks = append(tasks, v)
		}

		deployList, _ := h.getDeployTasks(tasks)
		deploys = append(deploys, deployList...)
	}

	for _, deploy := range deploys {
		if deploy.Enabled && !pt.ResetImage {
			if err := h.updateProductImageByNs(deploy.Namespace, deploy.ProductName, deploy.ServiceName, deploy.ContainerName, deploy.Image); err != nil {
				h.log.Errorf("updateProductImage %v error: %v", deploy, err)
				continue
			} else {
				h.log.Infof("succeed to update container image %v", deploy)
			}
		}
	}

	// 更新历史pipeline状态（默认留下前一百个）
	if err = h.ptColl.ArchiveHistoryPipelineTask(pt.PipelineName, pt.Type, 100); err != nil {
		h.log.Errorf("ArchiveHistoryPipelineTask error: %v", err)
	}

	return nil
}

func (h *TaskAckHandler) uploadTaskData(pt *task.Task) error {
	deliveryArtifacts := make([]*commonmodels.DeliveryArtifact, 0)
	if pt.Type == config.WorkflowType {
		stageArray := pt.Stages
		for _, subStage := range stageArray {
			taskType := subStage.TaskType
			taskStatus := subStage.Status
			switch taskType {
			case config.TaskBuild:
				if taskStatus == config.StatusPassed {
					subBuildTaskMap := subStage.SubTasks
					for _, subTask := range subBuildTaskMap {
						buildInfo, err := base.ToBuildTask(subTask)
						if err != nil {
							h.log.Errorf("uploadTaskData get buildInfo ToBuildTask failed ! err:%v", err)
							continue
						}
						deliveryArtifact := new(commonmodels.DeliveryArtifact)
						deliveryArtifact.CreatedBy = pt.TaskCreator
						deliveryArtifact.CreatedTime = time.Now().Unix()
						deliveryArtifact.Source = string(config.WorkflowType)
						if buildInfo.JobCtx.FileArchiveCtx != nil { // file
							deliveryArtifact.Name = buildInfo.ServiceName
							// TODO(Ray) file类型的交付物名称存放在Image和ImageTag字段是不规范的，优化时需要考虑历史数据的兼容问题。
							deliveryArtifact.Image = buildInfo.JobCtx.FileArchiveCtx.FileName
							deliveryArtifact.ImageTag = buildInfo.JobCtx.FileArchiveCtx.FileName
							deliveryArtifact.Type = string(config.File)
							deliveryArtifact.PackageFileLocation = buildInfo.JobCtx.FileArchiveCtx.FileLocation
							storageInfo, _ := s3.NewS3StorageFromEncryptedURI(pt.StorageURI)
							deliveryArtifact.PackageStorageURI = storageInfo.Endpoint

						} else if buildInfo.ServiceType != setting.PMDeployType { // image
							image := buildInfo.JobCtx.Image
							imageArray := strings.Split(image, "/")
							tagArray := strings.Split(imageArray[len(imageArray)-1], ":")
							imageName := tagArray[0]
							imageTag := tagArray[1]

							deliveryArtifact.Image = image
							deliveryArtifact.Type = string(config.Image)
							deliveryArtifact.Name = imageName
							deliveryArtifact.ImageTag = imageTag
							//获取镜像详细信息
							imageInfo, _ := getImageInfo(imageName, imageTag, h.log)
							if imageInfo != nil {
								deliveryArtifact.ImageSize = imageInfo.ImageSize
								deliveryArtifact.ImageDigest = imageInfo.ImageDigest
								deliveryArtifact.Architecture = imageInfo.Architecture
								deliveryArtifact.Os = imageInfo.Os
							}
							if dockerClient, err := h.getDockerClient(pt.DockerHost); err == nil {
								dockerHistories, err := dockerClient.ImageHistory(context.Background(), image)
								if err == nil && len(dockerHistories) > 0 {
									layers := make([]commonmodels.Descriptor, 0)
									for _, dockerHistory := range dockerHistories {
										var descriptor commonmodels.Descriptor
										descriptor.Digest = dockerHistory.ID
										descriptor.MediaType = dockerHistory.CreatedBy
										descriptor.Size = dockerHistory.Size

										layers = append(layers, descriptor)
									}
									deliveryArtifact.Layers = layers
								}
							}

							if buildInfo.JobCtx.DockerBuildCtx != nil {
								for _, build := range buildInfo.JobCtx.Builds {
									path := buildInfo.JobCtx.DockerBuildCtx.DockerFile
									pathArray := strings.Split(path, "/")
									dockerfilePath := path[len(pathArray[0])+1:]
									if strings.Contains(build.Address, "gitlab") {
										cli, err := gitlab.NewClient(build.Address, build.OauthToken)
										if err != nil {
											h.log.Errorf("Failed to get gitlab client, err: %v", err)
											continue
										}
										content, err := cli.GetRawFile(build.RepoOwner, build.RepoName, build.Branch, dockerfilePath)
										if err != nil {
											h.log.Errorf("uploadTaskData gitlab GetRawFile err:%v", err)
											continue
										}
										deliveryArtifact.DockerFile = string(content)
									} else {
										gitClient := git.NewClient(build.OauthToken, config.ProxyHTTPSAddr())
										fileContent, _, _, _ := gitClient.Repositories.GetContents(context.Background(), build.RepoOwner, build.RepoName, dockerfilePath, nil)
										if fileContent != nil {
											dockerfileContent := *fileContent.Content
											dockerfileContent = dockerfileContent[:len(dockerfileContent)-2]
											content, err := base64.StdEncoding.DecodeString(dockerfileContent)
											if err != nil {
												h.log.Errorf("uploadTaskData github GetRawFile err:%v", err)
												continue
											}
											deliveryArtifact.DockerFile = string(content)
										}
									}
								}
							}
						}
						tempDeliveryArtifacts, _, _ := h.deliveryArtifactColl.List(&commonrepo.DeliveryArtifactArgs{Name: deliveryArtifact.Name, Type: deliveryArtifact.Type, ImageTag: deliveryArtifact.ImageTag})
						if len(tempDeliveryArtifacts) == 0 {
							err = h.deliveryArtifactColl.Insert(deliveryArtifact)
							if err == nil {
								deliveryArtifacts = append(deliveryArtifacts, deliveryArtifact)
								//添加事件
								deliveryActivity := new(commonmodels.DeliveryActivity)
								deliveryActivity.Type = setting.BuildType
								deliveryActivity.ArtifactID = deliveryArtifact.ID
								deliveryActivity.URL = fmt.Sprintf("/v1/projects/detail/%s/pipelines/multi/%s/%d", pt.ProductName, pt.PipelineName, pt.TaskID)
								commits := make([]*commonmodels.ActivityCommit, 0)
								for _, build := range buildInfo.JobCtx.Builds {
									deliveryCommit := new(commonmodels.ActivityCommit)
									deliveryCommit.Address = build.Address
									deliveryCommit.Source = build.Source
									deliveryCommit.RepoOwner = build.RepoOwner
									deliveryCommit.RepoName = build.RepoName
									deliveryCommit.Branch = build.Branch
									deliveryCommit.Tag = build.Tag
									deliveryCommit.PR = build.PR
									deliveryCommit.CommitID = build.CommitID
									deliveryCommit.CommitMessage = build.CommitMessage
									deliveryCommit.AuthorName = build.AuthorName

									commits = append(commits, deliveryCommit)
								}
								deliveryActivity.Commits = commits

								issueURLs := make([]string, 0)
								//找到jira这个stage
								for _, jiraSubStage := range stageArray {
									if jiraSubStage.TaskType == config.TaskJira {
										jiraSubBuildTaskMap := jiraSubStage.SubTasks
										for _, jiraSubTask := range jiraSubBuildTaskMap {
											jiraInfo, _ := base.ToJiraTask(jiraSubTask)
											if jiraInfo != nil {
												for _, issue := range jiraInfo.Issues {
													issueURLs = append(issueURLs, issue.URL)
												}
												break
											}
										}
										break
									}
								}

								deliveryActivity.Issues = issueURLs
								deliveryActivity.CreatedBy = pt.TaskCreator
								deliveryActivity.CreatedTime = time.Now().Unix()
								deliveryActivity.StartTime = buildInfo.StartTime
								deliveryActivity.EndTime = buildInfo.EndTime

								err = h.deliveryActivityColl.Insert(deliveryActivity)
								if err != nil {
									h.log.Errorf("uploadTaskData build deliveryActivityColl insert err:%v", err)
									continue
								}
							}
						}
					}
				}
			case config.TaskDeploy:
				if taskStatus == config.StatusPassed {
					subDeployTaskMap := subStage.SubTasks
					for _, subTask := range subDeployTaskMap {
						deployInfo, err := base.ToDeployTask(subTask)
						if err != nil {
							h.log.Errorf("uploadTaskData get deployInfo ToDeployTask failed ! err:%v", err)
							continue
						}
						artifactID := h.getArtifactID(deployInfo.Image, deliveryArtifacts)
						if artifactID != "" {
							deliveryActivity := new(commonmodels.DeliveryActivity)
							deliveryActivity.Type = setting.DeployType
							deliveryActivity.CreatedBy = pt.TaskCreator
							deliveryActivity.URL = fmt.Sprintf("/v1/projects/detail/%s/pipelines/multi/%s/%d", pt.ProductName, pt.PipelineName, pt.TaskID)
							deliveryActivity.EnvName = deployInfo.EnvName
							deliveryActivity.Namespace = deployInfo.Namespace
							deliveryActivity.CreatedTime = time.Now().Unix() + 2
							deliveryActivity.StartTime = deployInfo.StartTime
							deliveryActivity.EndTime = deployInfo.EndTime

							err = h.deliveryActivityColl.Insert(deliveryActivity)
							if err != nil {
								h.log.Errorf("uploadTaskData deploy deliveryActivityColl insert err:%v", err)
								continue
							}
						}
					}
				}
			case config.TaskTestingV2:
				subTestTaskMap := subStage.SubTasks
				for _, subTask := range subTestTaskMap {
					var (
						isNew        = false
						testTaskStat *commonmodels.TestTaskStat
					)
					testInfo, err := base.ToTestingTask(subTask)
					if err != nil {
						h.log.Errorf("uploadTaskData get testInfo ToTestingTask failed ! err:%v", err)
						continue
					}

					if testInfo.JobCtx.TestType == setting.FunctionTestType {
						testTaskStat, _ = h.TestTaskStatColl.FindTestTaskStat(&commonrepo.TestTaskStatOption{Name: testInfo.TestModuleName})
						if testTaskStat == nil {
							isNew = true
							testTaskStat = new(commonmodels.TestTaskStat)
							testTaskStat.Name = testInfo.TestModuleName
							testTaskStat.CreateTime = time.Now().Unix()
							testTaskStat.UpdateTime = time.Now().Unix()
						}

						var filename string
						if storage, err := s3.FindDefaultS3(); err == nil {
							filename, err = util.GenerateTmpFile()
							if err != nil {
								h.log.Errorf("uploadTaskData GenerateTmpFile err:%v", err)
							}

							testJobName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
								config.WorkflowType, pt.PipelineName, pt.TaskID, config.TaskTestingV2, testInfo.TestModuleName)), "_", "-", -1)
							fileSrc := fmt.Sprintf("%s/%d/%s/%s", pt.PipelineName, pt.TaskID, "test", testJobName)
							forcedPathStyle := false
							if storage.Provider == setting.ProviderSourceSystemDefault {
								forcedPathStyle = true
							}
							client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Insecure, forcedPathStyle)
							objectKey := storage.GetObjectPath(fileSrc)
							err = client.Download(storage.Bucket, objectKey, filename)
							if err != nil {
								h.log.Errorf("uploadTaskData s3 Download err:%v", err)
							}
							_ = os.Remove(filename)
						} else {
							h.log.Errorf("uploadTaskData FindDefaultS3 err:%v", err)
						}

						b, err := ioutil.ReadFile(filename)
						if err != nil {
							msg := fmt.Sprintf("uploadTaskData get local test result file error: %v", err)
							h.log.Error(msg)
						}

						testReport := new(commonmodels.TestSuite)
						if err := xml.Unmarshal(b, &testReport); err != nil {
							msg := fmt.Sprintf("uploadTaskData testSuite unmarshal it report xml error: %v", err)
							h.log.Error(msg)
						}
						totalCaseNum := testReport.Tests
						if totalCaseNum != 0 {
							testTaskStat.TestCaseNum = totalCaseNum
						}
						testTaskStat.TotalDuration += testInfo.EndTime - testInfo.StartTime
					}

					if taskStatus == config.StatusPassed {
						if testInfo.JobCtx.TestType == setting.FunctionTestType {
							testTaskStat.TotalSuccess++
						}
						for _, deliveryArtifact := range deliveryArtifacts {
							deliveryActivity := new(commonmodels.DeliveryActivity)
							deliveryActivity.ArtifactID = deliveryArtifact.ID
							deliveryActivity.Type = setting.TestType
							deliveryActivity.CreatedBy = pt.TaskCreator
							deliveryActivity.URL = fmt.Sprintf("/v1/projects/detail/%s/pipelines/multi/%s/%d", pt.ProductName, pt.PipelineName, pt.TaskID)
							deliveryActivity.CreatedTime = time.Now().Unix() + 4
							deliveryActivity.StartTime = testInfo.StartTime
							deliveryActivity.EndTime = testInfo.EndTime

							err = h.deliveryActivityColl.Insert(deliveryActivity)
							if err != nil {
								h.log.Errorf("uploadTaskData test deliveryActivityColl insert err:%v", err)
								continue
							}
						}
					} else {
						if testInfo.JobCtx.TestType == setting.FunctionTestType {
							testTaskStat.TotalFailure++
						}
					}
					if testInfo.JobCtx.TestType == setting.FunctionTestType {
						if isNew { //新增
							_ = h.TestTaskStatColl.Create(testTaskStat)
						} else { //更新
							testTaskStat.UpdateTime = time.Now().Unix()
							_ = h.TestTaskStatColl.Update(testTaskStat)
						}
					}
				}
			case config.TaskReleaseImage:
				if taskStatus == config.StatusPassed {
					subDistributeTaskMap := subStage.SubTasks
					for _, subTask := range subDistributeTaskMap {
						releaseImageInfo, err := base.ToReleaseImageTask(subTask)
						if err != nil {
							h.log.Errorf("uploadTaskData get releaseImage ToReleaseImageTask failed ! err:%v", err)
							continue
						}
						artifactID := h.getArtifactID(releaseImageInfo.ImageTest, deliveryArtifacts)
						if artifactID != "" {
							deliveryActivity := new(commonmodels.DeliveryActivity)
							deliveryActivity.Type = setting.PublishType
							deliveryActivity.CreatedBy = pt.TaskCreator
							deliveryActivity.URL = fmt.Sprintf("/v1/projects/detail/%s/pipelines/multi/%s/%d", pt.ProductName, pt.PipelineName, pt.TaskID)
							publishHosts := make([]string, 0)
							publishNamespaces := make([]string, 0)
							for _, release := range releaseImageInfo.Releases {
								publishHosts = append(publishHosts, release.Host)
								publishNamespaces = append(publishNamespaces, release.Namespace)
							}
							deliveryActivity.PublishHosts = publishHosts
							deliveryActivity.PublishNamespaces = publishNamespaces
							deliveryActivity.CreatedTime = time.Now().Unix() + 6
							deliveryActivity.StartTime = releaseImageInfo.StartTime
							deliveryActivity.EndTime = releaseImageInfo.EndTime

							err = h.deliveryActivityColl.Insert(deliveryActivity)
							if err != nil {
								h.log.Errorf("uploadTaskData deploy deliveryActivityColl insert err:%v", err)
								continue
							}
						}
					}
				}
			case config.TaskDistributeToS3:
				if taskStatus == config.StatusPassed {
					subDistributeFileTaskMap := subStage.SubTasks
					for _, subTask := range subDistributeFileTaskMap {
						releaseFileInfo, err := base.ToDistributeToS3Task(subTask)
						if err != nil {
							h.log.Errorf("uploadTaskData get releaseFile ToDistributeToS3Task failed ! err:%v", err)
							continue
						}
						artifactID := h.getArtifactID(releaseFileInfo.PackageFile, deliveryArtifacts)
						if artifactID != "" {
							deliveryActivity := new(commonmodels.DeliveryActivity)
							deliveryActivity.Type = setting.PublishType
							deliveryActivity.CreatedBy = pt.TaskCreator
							deliveryActivity.URL = fmt.Sprintf("/v1/projects/detail/%s/pipelines/multi/%s/%d", pt.ProductName, pt.PipelineName, pt.TaskID)
							deliveryActivity.RemoteFileKey = releaseFileInfo.RemoteFileKey
							storageInfo, _ := s3.NewS3StorageFromEncryptedURI(releaseFileInfo.DestStorageURL)
							deliveryActivity.DistStorageURL = storageInfo.Endpoint
							storageInfo, _ = s3.NewS3StorageFromEncryptedURI(pt.StorageURI)
							deliveryActivity.SrcStorageURL = storageInfo.Endpoint
							deliveryActivity.CreatedTime = time.Now().Unix() + 6
							deliveryActivity.StartTime = releaseFileInfo.StartTime
							deliveryActivity.EndTime = releaseFileInfo.EndTime

							err = h.deliveryActivityColl.Insert(deliveryActivity)
							if err != nil {
								h.log.Errorf("uploadTaskData releaseFile deliveryActivityColl insert err:%v", err)
								continue
							}
						}
					}
				}
			}
		}
	} else if pt.Type == config.TestType {
		stageArray := pt.Stages
		for _, subStage := range stageArray {
			taskType := subStage.TaskType
			taskStatus := subStage.Status
			switch taskType {
			case config.TaskTestingV2:
				subTestTaskMap := subStage.SubTasks
				for _, subTask := range subTestTaskMap {
					testInfo, err := base.ToTestingTask(subTask)
					if err != nil {
						h.log.Errorf("uploadTaskData get testInfo ToTestingTask failed ! err:%v", err)
						continue
					}

					if testInfo.JobCtx.TestType == setting.FunctionTestType {
						isNew := false
						testTaskStat, _ := h.TestTaskStatColl.FindTestTaskStat(&commonrepo.TestTaskStatOption{Name: testInfo.TestModuleName})
						if testTaskStat == nil {
							isNew = true
							testTaskStat = new(commonmodels.TestTaskStat)
							testTaskStat.Name = testInfo.TestModuleName
							testTaskStat.CreateTime = time.Now().Unix()
							testTaskStat.UpdateTime = time.Now().Unix()
						}

						var filename string
						if storage, err := s3.FindDefaultS3(); err == nil {
							filename, err = util.GenerateTmpFile()
							if err != nil {
								h.log.Errorf("uploadTaskData GenerateTmpFile err:%v", err)
							}

							pipelineName := fmt.Sprintf("%s-%s", testInfo.TestModuleName, "job")
							testJobName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
								config.TestType, pipelineName, pt.TaskID, config.TaskTestingV2, testInfo.TestModuleName)), "_", "-", -1)
							fileSrc := fmt.Sprintf("%s/%d/%s/%s", pipelineName, pt.TaskID, "test", testJobName)
							forcedPathStyle := false
							if storage.Provider == setting.ProviderSourceSystemDefault {
								forcedPathStyle = true
							}
							client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Insecure, forcedPathStyle)
							objectKey := storage.GetObjectPath(fileSrc)
							err = client.Download(storage.Bucket, objectKey, filename)
							if err != nil {
								h.log.Errorf("uploadTaskData s3 Download err:%v", err)
							}
							_ = os.Remove(filename)
						} else {
							h.log.Errorf("uploadTaskData FindDefaultS3 err:%v", err)
						}

						b, err := ioutil.ReadFile(filename)
						if err != nil {
							msg := fmt.Sprintf("uploadTaskData get local test result file error: %v", err)
							h.log.Error(msg)
						}

						testReport := new(commonmodels.TestSuite)
						if err := xml.Unmarshal(b, &testReport); err != nil {
							msg := fmt.Sprintf("uploadTaskData testSuite unmarshal it report xml error: %v", err)
							h.log.Error(msg)
						}
						totalCaseNum := testReport.Tests
						if totalCaseNum != 0 {
							testTaskStat.TestCaseNum = totalCaseNum
						}
						testTaskStat.TotalDuration += testInfo.EndTime - testInfo.StartTime

						if taskStatus == config.StatusPassed {
							testTaskStat.TotalSuccess++
						} else {
							testTaskStat.TotalFailure++
						}
						if isNew { //新增
							_ = h.TestTaskStatColl.Create(testTaskStat)
						} else { //更新
							testTaskStat.UpdateTime = time.Now().Unix()
							_ = h.TestTaskStatColl.Update(testTaskStat)
						}
					}
				}
			}
		}
	}
	return nil
}

func (h *TaskAckHandler) createVersion(pt *task.Task) error {
	if pt.Status == config.StatusPassed && pt.Type == config.WorkflowType {
		if pt.WorkflowArgs != nil && pt.WorkflowArgs.VersionArgs != nil && pt.WorkflowArgs.VersionArgs.Enabled {
			stageArray := pt.Stages
			isDeploy := false
			for _, subStage := range stageArray {
				if subStage.TaskType == config.TaskDeploy {
					isDeploy = true
					break
				}
			}
			if isDeploy {
				//版本交付
				return commonservice.AddDeliveryVersion(1, int(pt.TaskID), pt.ProductName, pt.PipelineName, pt, log.SugaredLogger())
			}
		}
	}
	return nil
}

func (h *TaskAckHandler) updateWorkflowStat(pt *task.Task) error {
	if pt.IsRestart {
		return nil
	}
	if pt.Type == config.SingleType || pt.Type == config.WorkflowType {
		totalSuccess := 0
		totalFailure := 0
		if pt.Status == config.StatusPassed {
			totalSuccess = 1
			totalFailure = 0
		} else {
			totalSuccess = 0
			totalFailure = 1
		}
		workflowStat := &commonmodels.WorkflowStat{
			ProductName:   pt.ProductName,
			Name:          pt.PipelineName,
			Type:          string(pt.Type),
			TotalDuration: pt.EndTime - pt.StartTime,
			TotalSuccess:  totalSuccess,
			TotalFailure:  totalFailure,
		}
		if err := upsertWorkflowStat(workflowStat, h.log); err != nil {
			return err
		}

	}

	return nil
}

func (h *TaskAckHandler) getArtifactID(image string, deliveryArtifacts []*commonmodels.DeliveryArtifact) string {
	for _, deliveryArtifact := range deliveryArtifacts {
		if deliveryArtifact.Image == image {
			return deliveryArtifact.ID.Hex()
		}
	}
	return ""
}

func (h *TaskAckHandler) getDockerClient(dockerHostURL string) (*dockerCli.Client, error) {
	//dockerHostURL tcp://dind-1.dind:2375
	url, err := dockerCli.ParseHostURL(dockerHostURL)
	if err != nil {
		return nil, err
	}
	transport := new(http.Transport)
	err = sockets.ConfigureTransport(transport, url.Scheme, url.Host)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Transport:     transport,
		CheckRedirect: dockerCli.CheckRedirect,
	}

	cli, err := dockerCli.NewClient(dockerHostURL, "1.37", httpClient, map[string]string{})
	return cli, err
}

// 获取subtasks中的所有容器部署任务
func (h *TaskAckHandler) getDeployTasks(subTasks []map[string]interface{}) ([]*task.Deploy, error) {

	deploys := make([]*task.Deploy, 0)
	for _, subTask := range subTasks {

		pre, err := base.ToPreview(subTask)
		if err != nil {
			return nil, errors.New("invalid sub task type")
		}

		switch pre.TaskType {

		case config.TaskDeploy:
			deploy, err := base.ToDeployTask(subTask)
			if err != nil {
				return nil, fmt.Errorf("unmarshal deploy sub task type error: %v", err)
			}

			// deploy有状态的时候就更新, 不管是否成功
			if deploy.Enabled && deploy.TaskStatus != "" {
				deploys = append(deploys, deploy)
			}
		}
	}

	if len(deploys) == 0 {
		return nil, errors.New("no deploy sub task found")
	}

	return deploys, nil
}

// 更新subtasks中的所有容器部署任务对应服务的镜像
func (h *TaskAckHandler) updateProductImageByNs(namespace, productName, serviceName, containerName, imageName string) error {
	opt := &commonrepo.ProductEnvFindOptions{
		Name:      productName,
		Namespace: namespace,
	}

	prod, err := h.productColl.FindEnv(opt)

	if err != nil {
		h.log.Errorf("find product namespace error: %v", err)
		return err
	}

	for i, group := range prod.Services {
		for j, service := range group {
			if service.ServiceName == serviceName {
				for l, container := range service.Containers {
					if container.Name == containerName {
						prod.Services[i][j].Containers[l].Image = imageName
					}
				}
			}
		}
	}

	if err := h.productColl.Update(prod); err != nil {
		errMsg := fmt.Sprintf("[%s][%s] update product image error: %v", prod.EnvName, prod.ProductName, err)
		h.log.Errorf(errMsg)
		return errors.New(errMsg)
	}

	return nil
}

type ItReportHandler struct {
	itReportColl *commonrepo.ItReportColl
	log          *zap.SugaredLogger
}

func (h *ItReportHandler) HandleMessage(message *nsq.Message) error {

	var report *commonmodels.ItReport
	if err := json.Unmarshal(message.Body, &report); err != nil {
		h.log.Errorf("unmarshal ItReport message error: %v", err)
		return nil
	}

	h.log.Infof("receive it report: %+v", report)

	if err := h.itReportColl.Upsert(report); err != nil {
		h.log.Errorf("create ItReport error: %v", err)
	}
	return nil
}

// TaskNotificationHandler ...
type TaskNotificationHandler struct {
	log *zap.SugaredLogger
}

// HandleMessage ...
func (h *TaskNotificationHandler) HandleMessage(message *nsq.Message) error {
	var n *commonmodels.Notify
	if err := json.Unmarshal(message.Body, &n); err != nil {
		h.log.Errorf("unmarshal Notify message error: %v", err)
		return nil
	}

	h.log.Infof("receive notification: %+v", n)

	notifyClient := notify.NewNotifyClient()
	if err := notifyClient.ProccessNotify(n); err != nil {
		h.log.Errorf("send notify error :%v", err)
	}

	return nil
}

func upsertWorkflowStat(args *commonmodels.WorkflowStat, log *zap.SugaredLogger) error {
	if workflowStats, _ := commonrepo.NewWorkflowStatColl().FindWorkflowStat(&commonrepo.WorkflowStatArgs{Name: args.Name, Type: args.Type}); len(workflowStats) > 0 {
		currentWorkflowStat := workflowStats[0]
		currentWorkflowStat.TotalDuration += args.TotalDuration
		currentWorkflowStat.TotalSuccess += args.TotalSuccess
		currentWorkflowStat.TotalFailure += args.TotalFailure
		if err := commonrepo.NewWorkflowStatColl().Upsert(currentWorkflowStat); err != nil {
			log.Errorf("WorkflowStat upsert err:%v", err)
			return err
		}
	} else {
		args.CreatedAt = time.Now().Unix()
		args.UpdatedAt = time.Now().Unix()
		if err := commonrepo.NewWorkflowStatColl().Create(args); err != nil {
			log.Errorf("WorkflowStat create err:%v", err)
			return err
		}
	}

	return nil
}

func getImageInfo(repoName, tag string, log *zap.SugaredLogger) (*commonmodels.DeliveryImage, error) {
	regOps := new(commonrepo.FindRegOps)
	regOps.IsDefault = true
	registryInfo, err := commonrepo.NewRegistryNamespaceColl().Find(regOps)
	if err != nil {
		log.Errorf("RegistryNamespace.get error: %v", err)
		return nil, fmt.Errorf("RegistryNamespace.get error: %v", err)
	}

	return registry.NewV2Service(registryInfo.RegProvider).GetImageInfo(registry.GetRepoImageDetailOption{
		Endpoint: registry.Endpoint{
			Addr:      registryInfo.RegAddr,
			Ak:        registryInfo.AccessKey,
			Sk:        registryInfo.SecretKey,
			Namespace: registryInfo.Namespace,
			Region:    registryInfo.Region,
		},
		Image: repoName,
		Tag:   tag,
	}, log)
}
