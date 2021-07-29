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

package scmnotify

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util"
)

type Service struct {
	Client       *Client
	Coll         *mongodb.NotificationColl
	DiffNoteColl *mongodb.DiffNoteColl
}

func NewService() *Service {
	return &Service{
		Client:       NewClient(),
		Coll:         mongodb.NewNotificationColl(),
		DiffNoteColl: mongodb.NewDiffNoteColl(),
	}
}

func (s *Service) SendInitWebhookComment(
	mainRepo *models.MainHookRepo, prID int, baseURI string, isPipeline, isTest bool, logger *zap.SugaredLogger,
) (*models.Notification, error) {
	notification := &models.Notification{
		CodehostID: mainRepo.CodehostID,
		PrID:       prID,
		ProjectID:  strings.TrimLeft(mainRepo.RepoOwner+"/"+mainRepo.RepoName, "/"),
		BaseURI:    baseURI,
		IsPipeline: isPipeline,
		IsTest:     isTest,
		Label:      mainRepo.GetLabelValue(),
		Revision:   mainRepo.Revision,
	}

	if err := s.Client.Comment(notification); err != nil {
		logger.Errorf("failed to comment to %s %v", notification.ToString(), err)
		return nil, err
	} else if err := s.Coll.Create(notification); err != nil {
		logger.Errorf("failed to save %s %v", notification.ToString(), err)
		return nil, err
	}

	return notification, nil
}

func (s *Service) SendErrWebhookComment(
	mainRepo *models.MainHookRepo, workflow *models.Workflow, err error, prID int, baseURI string, isPipeline, isTest bool, logger *zap.SugaredLogger,
) (*models.Notification, error) {
	_, message := e.ErrorMessage(err)
	errStr := "创建工作流任务失败"
	if description, ok := message["description"]; ok {
		if description != nil {
			if desc, ok := description.(string); ok {
				errStr = desc
			}
		}
	}

	url := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/multi/%s", baseURI, workflow.ProductTmplName, workflow.Name)
	errInfo := fmt.Sprintf("创建工作流任务失败，项目名称：%s， 工作流名称：[%s](%s)， 错误信息：%s", workflow.ProductTmplName, workflow.Name, url, errStr)

	notification := &models.Notification{
		CodehostID: mainRepo.CodehostID,
		PrID:       prID,
		ProjectID:  strings.TrimLeft(mainRepo.RepoOwner+"/"+mainRepo.RepoName, "/"),
		BaseURI:    baseURI,
		IsPipeline: isPipeline,
		IsTest:     isTest,
		ErrInfo:    errInfo,
		Label:      mainRepo.GetLabelValue(),
		Revision:   mainRepo.Revision,
	}

	if err := s.Client.Comment(notification); err != nil {
		logger.Errorf("failed to comment to %s %v", notification.ToString(), err)
		return nil, err
	} else if err := s.Coll.Create(notification); err != nil {
		logger.Errorf("failed to save %s %v", notification.ToString(), err)
		return nil, err
	}

	return notification, nil
}

func convertTaskStatusToNotificationTaskStatus(status config.Status) config.TaskStatus {
	switch status {
	case config.StatusCreated:
		fallthrough
	case config.StatusWaiting:
		fallthrough
	case config.StatusQueued:
		fallthrough
	case config.StatusBlocked:
		fallthrough
	case config.QueueItemPending:
		return config.TaskStatusReady
	case config.StatusRunning:
		return config.TaskStatusRunning
	case config.StatusFailed:
		return config.TaskStatusFailed
	case config.StatusTimeout:
		return config.TaskStatusTimeout
	case config.StatusCancelled:
		return config.TaskStatusCancelled
	case config.StatusPassed:
		return config.TaskStatusPass
	case config.StatusDisabled:
		fallthrough
	case config.StatusSkipped:
		return config.TaskStatusCompleted
	default:
		return config.TaskStatusReady
	}
}

func convertStatus(status string) string {
	switch status {
	case "Unknown":
		return "未知状态"
	case "Creating":
		return "创建中"
	case "Running":
		return "运行中"
	case "Deleting":
		return "删除中"
	case "Unstable":
		return "运行不稳定"
	case "Completed":
		return "删除完成"
	default:
		return "准备中"
	}
}

// updateWebhookComment update the comment to codehost when task status changes
func (s *Service) UpdateWebhookComment(task *task.Task, logger *zap.SugaredLogger) (err error) {
	if task.WorkflowArgs.NotificationID == "" {
		return
	}

	var notification *models.Notification
	if notification, err = s.Coll.Find(task.WorkflowArgs.NotificationID); err != nil {
		logger.Errorf("can't find notification by id %s %s", task.WorkflowArgs.NotificationID, err)
		return err
	}

	var tasks []*models.NotificationTask
	var taskExist bool
	var shouldComment bool
	status := convertTaskStatusToNotificationTaskStatus(task.Status)
	for _, nTask := range notification.Tasks {
		if nTask.ID == task.TaskID {
			shouldComment = nTask.Status != status
			scmTask := &models.NotificationTask{
				ProductName:  task.ProductName,
				WorkflowName: task.PipelineName,
				ID:           task.TaskID,
				Status:       status,
			}

			if status == config.TaskStatusPass {
				// 从s3获取测试报告数据
				testReports, err := DownloadTestReports(task, logger)
				if err != nil {
					logger.Errorf("download testReport from s3 failed,err:%v", err)
				}
				scmTask.TestReports = testReports
			}

			tasks = append(tasks, scmTask)
			taskExist = true
		} else {
			tasks = append(tasks, nTask)
		}
	}

	if !taskExist {
		tasks = append(tasks, &models.NotificationTask{
			ProductName:  task.ProductName,
			WorkflowName: task.PipelineName,
			ID:           task.TaskID,
			Status:       status,
		})
		shouldComment = true
	}

	if shouldComment {
		notification.Tasks = tasks
		if err = s.Client.Comment(notification); err != nil {
			logger.Errorf("failed to comment %s, %v", notification.ToString(), err)
		}

		if err = s.Coll.Upsert(notification); err != nil {
			logger.Errorf("can't upsert notification by id %s", notification.ID)
			return
		}
	} else {
		logger.Infof("status not changed of task %s %d, skip to update comment", task.PipelineName, task.TaskID)
	}

	return nil
}

// UpdateDiffNote 调用gitlab接口更新DiffNote，并更新到数据库
func (s *Service) UpdateDiffNote(task *task.Task, logger *zap.SugaredLogger) (err error) {
	if task.WorkflowArgs.NotificationID == "" {
		return
	}

	var notification *models.Notification
	if notification, err = s.Coll.Find(task.WorkflowArgs.NotificationID); err != nil {
		logger.Errorf("can't find notification by id %s %s", task.WorkflowArgs.NotificationID, err)
		return err
	}

	isAllTaskSucceed := true
	for _, nTask := range notification.Tasks {
		if nTask.Status != config.TaskStatusFailed && nTask.Status != config.TaskStatusCancelled &&
			nTask.Status != config.TaskStatusPass && nTask.Status != config.TaskStatusTimeout {
			// 存在任务没有执行完，直接返回
			return nil
		}
		// 任务都执行完了，确认是否有未成功的任务
		if nTask.Status != config.TaskStatusPass {
			isAllTaskSucceed = false
			break
		}
	}

	body := "KodeRover CI 检查通过"
	if !isAllTaskSucceed {
		body = "KodeRover CI 检查失败"
	}

	opt := &mongodb.DiffNoteFindOpt{
		CodehostID:     notification.CodehostID,
		ProjectID:      notification.ProjectID,
		MergeRequestID: notification.PrID,
	}
	diffNote, err := s.DiffNoteColl.Find(opt)
	if err != nil {
		logger.Errorf("can't find notification by id %s %v", task.WorkflowArgs.NotificationID, err)
		return err
	}

	cli, _ := gitlab.NewOAuthClient(diffNote.Repo.OauthToken, gitlab.WithBaseURL(diffNote.Repo.Address))

	// 更新note body
	noteBodyOpt := &gitlab.UpdateMergeRequestDiscussionNoteOptions{
		Body: &body,
	}
	_, _, err = cli.Discussions.UpdateMergeRequestDiscussionNote(diffNote.Repo.ProjectID, diffNote.MergeRequestID, diffNote.DiscussionID, diffNote.NoteID, noteBodyOpt)
	if err != nil {
		logger.Errorf("UpdateMergeRequestDiscussionNote failed, err: %v", err)
		return err
	}

	// 更新resolved状态
	resolveOpt := &gitlab.UpdateMergeRequestDiscussionNoteOptions{
		Resolved: &isAllTaskSucceed,
	}
	_, _, err = cli.Discussions.UpdateMergeRequestDiscussionNote(diffNote.Repo.ProjectID, diffNote.MergeRequestID, diffNote.DiscussionID, diffNote.NoteID, resolveOpt)
	if err != nil {
		logger.Errorf("UpdateMergeRequestDiscussionNote failed, err: %v", err)
		return err
	}

	diffNote.Resolved = isAllTaskSucceed
	diffNote.Body = body
	err = s.DiffNoteColl.Update(diffNote.ObjectID.Hex(), "", diffNote.Body, diffNote.Resolved)
	if err != nil {
		logger.Errorf("UpdateDiscussionInfo failed, err: %v", err)
		return err
	}

	return nil
}

func downloadReport(taskInfo *task.Task, fileName, testName string, logger *zap.SugaredLogger) (*models.TestSuite, error) {
	var store *s3.S3
	var err error

	if store, err = s3.NewS3StorageFromEncryptedURI(taskInfo.StorageURI); err != nil {
		logger.Errorf("failed to create s3 storage %s", taskInfo.StorageURI)
		return nil, err
	}
	if store.Subfolder != "" {
		store.Subfolder = fmt.Sprintf("%s/%s/%d/%s", store.Subfolder, taskInfo.PipelineName, taskInfo.TaskID, "test")
	} else {
		store.Subfolder = fmt.Sprintf("%s/%d/%s", taskInfo.PipelineName, taskInfo.TaskID, "test")
	}

	tmpFilename, _ := util.GenerateTmpFile()
	defer func() {
		_ = os.Remove(tmpFilename)
	}()

	objectKey := store.GetObjectPath(fileName)
	forcedPathStyle := false
	if store.Provider == setting.ProviderSourceSystemDefault {
		forcedPathStyle = true
	}
	client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Insecure, forcedPathStyle)
	if err != nil {
		return nil, err
	}

	err = client.Download(store.Bucket, objectKey, tmpFilename)
	if err != nil {
		logger.Errorf("Failed to download object: %s, error is: %+v", objectKey, err)
		return nil, err
	}

	testRepo := new(models.TestSuite)
	b, err := ioutil.ReadFile(tmpFilename)
	if err != nil {
		logger.Error(fmt.Sprintf("get test result file error: %v", err))
		return nil, err
	}

	err = xml.Unmarshal(b, testRepo)
	if err != nil {
		logger.Errorf("unmarshal result file test suite summary error: %v", err)
		return nil, err
	}

	testRepo.Name = testName

	return testRepo, nil
}

func DownloadTestReports(taskInfo *task.Task, logger *zap.SugaredLogger) ([]*models.TestSuite, error) {
	if taskInfo.StorageURI == "" {
		return nil, nil
	}

	testReport := make([]*models.TestSuite, 0)

	switch taskInfo.Type {
	case config.SingleType:
		//testName := taskInfo.TaskArgs.Test.TestModuleName
		fileName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s", config.SingleType,
			taskInfo.PipelineName, taskInfo.TaskID, config.TaskTestingV2, taskInfo.ServiceName)), "_", "-", -1)
		testRepo, err := downloadReport(taskInfo, fileName, taskInfo.ServiceName, logger)
		if err != nil {
			return nil, err
		}
		testReport = append(testReport, testRepo)
		return testReport, nil
	case config.WorkflowType:
		if taskInfo.WorkflowArgs == nil {
			return nil, nil
		}
		for _, test := range taskInfo.WorkflowArgs.Tests {
			fileName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
				config.WorkflowType, taskInfo.PipelineName, taskInfo.TaskID, config.TaskTestingV2, test.TestModuleName)), "_", "-", -1)
			testRepo, err := downloadReport(taskInfo, fileName, test.TestModuleName, logger)
			if err != nil {
				return nil, err
			}
			testReport = append(testReport, testRepo)
		}
		return testReport, nil
	case config.TestType:
		if taskInfo.TestArgs == nil {
			return nil, nil
		}
		testName := taskInfo.TestArgs.TestName
		fileName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
			config.TestType, taskInfo.PipelineName, taskInfo.TaskID, config.TaskTestingV2, testName)), "_", "-", -1)
		testRepo, err := downloadReport(taskInfo, fileName, testName, logger)
		if err != nil {
			return nil, err
		}
		testReport = append(testReport, testRepo)
		return testReport, nil
	}

	return nil, nil
}

// UpdateWebhookCommentForTest update the test comment to codehost when task status changes
func (s *Service) UpdateWebhookCommentForTest(task *task.Task, logger *zap.SugaredLogger) (err error) {
	if task.TestArgs.NotificationID == "" {
		return
	}

	var notification *models.Notification
	if notification, err = s.Coll.Find(task.TestArgs.NotificationID); err != nil {
		logger.Errorf("can't find notification by id %s %s", task.TestArgs.NotificationID, err)
		return err
	}

	var tasks []*models.NotificationTask
	var taskExist bool
	var shouldComment bool

	status := convertTaskStatusToNotificationTaskStatus(task.Status)
	for _, nTask := range notification.Tasks {
		if nTask.ID == task.TaskID {
			shouldComment = nTask.Status != status
			scmTask := &models.NotificationTask{
				ProductName: task.ProductName,
				TestName:    task.PipelineName,
				ID:          task.TaskID,
				Status:      status,
			}

			if status == config.TaskStatusPass {
				// 从s3获取测试报告数据
				testReports, err := DownloadTestReports(task, logger)
				if err != nil {
					logger.Errorf("download testReport from s3 failed,err:%v", err)
				}
				scmTask.TestReports = testReports
			}

			tasks = append(tasks, scmTask)
			taskExist = true
		} else {
			tasks = append(tasks, nTask)
		}
	}

	if !taskExist {
		tasks = append(tasks, &models.NotificationTask{
			ProductName: task.ProductName,
			TestName:    task.PipelineName,
			ID:          task.TaskID,
			Status:      status,
		})
		shouldComment = true
	}

	if shouldComment {
		notification.Tasks = tasks
		if err = s.Client.Comment(notification); err != nil {
			logger.Errorf("failed to comment %s, %v", notification.ToString(), err)
		}

		if err = s.Coll.Upsert(notification); err != nil {
			logger.Errorf("can't upsert notification by id %s", notification.ID)
			return
		}
	} else {
		logger.Infof("status not changed of task %s %d, skip to update comment", task.PipelineName, task.TaskID)
	}

	return nil
}

func (s *Service) UpdatePipelineWebhookComment(task *task.Task, logger *zap.SugaredLogger) (err error) {
	if task.TaskArgs == nil {
		logger.Warnf("taskArgs of %s is nil", task.PipelineName)
		return
	}

	if task.TaskArgs.NotificationID == "" {
		return
	}

	var notification *models.Notification
	if notification, err = s.Coll.Find(task.TaskArgs.NotificationID); err != nil {
		logger.Errorf("can't find notification by id %s %s", task.TaskArgs.NotificationID, err)
		return err
	}

	var tasks []*models.NotificationTask
	var taskExist bool
	var shouldComment bool

	status := convertTaskStatusToNotificationTaskStatus(task.Status)
	for _, nTask := range notification.Tasks {
		if nTask.ID == task.TaskID {
			shouldComment = nTask.Status != status
			scmTask := &models.NotificationTask{
				ProductName:  task.ProductName,
				PipelineName: task.PipelineName,
				ID:           task.TaskID,
				Status:       status,
			}

			if status == config.TaskStatusPass {
				// 从s3获取测试报告数据
				testReports, err := DownloadTestReports(task, logger)
				if err != nil {
					logger.Errorf("download testReport from s3 failed,err:%v", err)
				}
				scmTask.TestReports = testReports
			}

			tasks = append(tasks, scmTask)
			taskExist = true
		} else {
			tasks = append(tasks, nTask)
		}
	}

	if !taskExist {
		tasks = append(tasks, &models.NotificationTask{
			ProductName:  task.ProductName,
			PipelineName: task.PipelineName,
			ID:           task.TaskID,
			Status:       status,
		})
		shouldComment = true
	}
	if shouldComment {
		notification.Tasks = tasks
		if err = s.Coll.Upsert(notification); err != nil {
			logger.Errorf("can't upsert notification by id %s", notification.ID)
			return
		}

		if err = s.Client.Comment(notification); err != nil {
			logger.Errorf("failed to comment %s, %v", notification.ToString(), err)
		}
	} else {
		logger.Infof("status not changed of task %s %d, skip to update comment", task.PipelineName, task.TaskID)
	}

	return nil
}

func (s *Service) UpdateEnvAndTaskWebhookComment(workflowArgs *models.WorkflowTaskArgs, prTaskInfo *models.PrTaskInfo, logger *zap.SugaredLogger) (err error) {
	if workflowArgs.NotificationID == "" {
		return
	}

	var notification *models.Notification
	if notification, err = s.Coll.Find(workflowArgs.NotificationID); err != nil {
		logger.Errorf("UpdateEnvAndTaskWebhookComment can't find notification by id %s %s", workflowArgs.NotificationID, err)
		return err
	}
	shouldComment := false
	if notification.PrTask == nil {
		shouldComment = true
	} else {
		shouldComment = prTaskInfo.EnvStatus != notification.PrTask.EnvStatus
	}
	//转换状态
	if shouldComment {
		prTaskInfo.EnvStatus = convertStatus(prTaskInfo.EnvStatus)
		notification.PrTask = prTaskInfo
		if err = s.Coll.Upsert(notification); err != nil {
			logger.Errorf("UpdateEnvAndTaskWebhookComment can't upsert notification by id %s", notification.ID)
			return
		}

		if err = s.Client.Comment(notification); err != nil {
			logger.Errorf("UpdateEnvAndTaskWebhookComment failed to comment %s, %v", notification.ToString(), err)

		}
	} else {
		logger.Infof("UpdateEnvAndTaskWebhookComment status not changed of env %s, skip to update comment", prTaskInfo.EnvName)
	}

	return nil
}
