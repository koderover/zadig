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

	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
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
	mainRepo *models.MainHookRepo, prID int, baseURI string, isPipeline, isTest, isScanning, isWorkflowV4 bool, logger *zap.SugaredLogger,
) (*models.Notification, error) {
	notification := &models.Notification{
		CodehostID:   mainRepo.CodehostID,
		PrID:         prID,
		ProjectID:    strings.TrimLeft(mainRepo.GetRepoNamespace()+"/"+mainRepo.RepoName, "/"),
		BaseURI:      baseURI,
		IsPipeline:   isPipeline,
		IsTest:       isTest,
		IsScanning:   isScanning,
		IsWorkflowV4: isWorkflowV4,
		Label:        mainRepo.GetLabelValue(),
		Revision:     mainRepo.Revision,
		RepoOwner:    mainRepo.RepoOwner,
		RepoName:     mainRepo.RepoName,
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

	url := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/multi/%s?display_name=%s", baseURI, workflow.ProductTmplName, workflow.Name, workflow.DisplayName)
	errInfo := fmt.Sprintf("创建工作流任务失败，项目名称：%s， 工作流名称：[%s](%s)， 错误信息：%s", workflow.ProductTmplName, workflow.Name, url, errStr)

	notification := &models.Notification{
		CodehostID: mainRepo.CodehostID,
		PrID:       prID,
		ProjectID:  strings.TrimLeft(mainRepo.GetRepoNamespace()+"/"+mainRepo.RepoName, "/"),
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
				ProductName:         task.ProductName,
				WorkflowName:        task.PipelineName,
				WorkflowDisplayName: task.PipelineDisplayName,
				ID:                  task.TaskID,
				Status:              status,
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
	forcedPathStyle := true
	if store.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
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

func (s *Service) UpdateWebhookCommentForScanning(task *task.Task, logger *zap.SugaredLogger) (err error) {
	if task.ScanningArgs.NotificationID == "" {
		return
	}

	var notification *models.Notification
	if notification, err = s.Coll.Find(task.ScanningArgs.NotificationID); err != nil {
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
				ProductName:  task.ProductName,
				TestName:     task.PipelineName,
				ID:           task.TaskID,
				ScanningName: task.ScanningArgs.ScanningName,
				ScanningID:   task.ScanningArgs.ScanningID,
				Status:       status,
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
			TestName:     task.PipelineName,
			ID:           task.TaskID,
			ScanningName: task.ScanningArgs.ScanningName,
			ScanningID:   task.ScanningArgs.ScanningID,
			Status:       status,
		})
		shouldComment = true
	}

	if shouldComment {
		notification.Tasks = tasks
		if err = s.Client.Comment(notification); err != nil {
			// cannot return error here since the upsert operation is required for further use.
			logger.Warnf("failed to comment %s, %v", notification.ToString(), err)
		}

		if err = s.Coll.Upsert(notification); err != nil {
			logger.Warnf("can't upsert notification by id %s", notification.ID)
			return
		}
	} else {
		logger.Infof("status not changed of task %s %d, skip to update comment", task.PipelineName, task.TaskID)
	}

	return nil
}

func (s *Service) UpdateWebhookCommentForWorkflowV4(task *models.WorkflowTask, logger *zap.SugaredLogger) (err error) {
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
				ProductName:  task.ProjectName,
				WorkflowName: task.WorkflowName,
				ID:           task.TaskID,
				Status:       status,
			}

			tasks = append(tasks, scmTask)
			taskExist = true
		} else {
			tasks = append(tasks, nTask)
		}
	}

	if !taskExist {
		tasks = append(tasks, &models.NotificationTask{
			ProductName:         task.ProjectName,
			WorkflowName:        task.WorkflowName,
			ID:                  task.TaskID,
			WorkflowDisplayName: task.WorkflowDisplayName,
			Status:              status,
		})
		shouldComment = true
	}

	if shouldComment {
		notification.Tasks = tasks
		if err = s.Client.Comment(notification); err != nil {
			// cannot return error here since the upsert operation is required for further use.
			logger.Warnf("failed to comment %s, %v", notification.ToString(), err)
		}

		if err = s.Coll.Upsert(notification); err != nil {
			logger.Warnf("can't upsert notification by id %s", notification.ID)
			return
		}
	} else {
		logger.Infof("status not changed of task %s %d, skip to update comment", task.WorkflowName, task.TaskID)
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

func (s *Service) CreateGitCheckForWorkflowV4(workflowArgs *models.WorkflowV4, taskID int64, log *zap.SugaredLogger) error {
	hook := workflowArgs.HookPayload

	if hook == nil || !hook.IsPr {
		return nil
	}

	ghApp, err := github.GetGithubAppClientByOwner(hook.Owner)
	if err != nil {
		log.Errorf("getGithubAppClient failed, err:%v", err)
		return e.ErrGithubUpdateStatus.AddErr(err)
	}
	if ghApp != nil {
		log.Infof("GitHub App found, start to create check-run")
		opt := &github.GitCheck{
			Owner:  hook.Owner,
			Repo:   hook.Repo,
			Branch: hook.Ref,
			Ref:    hook.Ref,
			IsPr:   hook.IsPr,

			AslanURL:    configbase.SystemAddress(),
			PipeName:    workflowArgs.Name,
			DisplayName: getDisplayName(workflowArgs),
			ProductName: workflowArgs.Project,
			PipeType:    config.WorkflowTypeV4,
			TaskID:      taskID,
		}
		checkID, err := ghApp.StartGitCheck(opt)
		if err != nil {
			return err
		}
		workflowArgs.HookPayload.CheckRunID = checkID
		return nil
	}

	log.Infof("Init GitHub status")
	ch, err := systemconfig.New().GetCodeHost(hook.CodehostID)
	if err != nil {
		log.Errorf("Failed to get codeHost, err:%v", err)
		return e.ErrGithubUpdateStatus.AddErr(err)
	}
	gc := github.NewClient(ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)

	return gc.UpdateCheckStatus(&github.StatusOptions{
		Owner:       hook.Owner,
		Repo:        hook.Repo,
		Ref:         hook.Ref,
		State:       github.StatePending,
		Description: fmt.Sprintf("Workflow [%s] is queued.", workflowArgs.DisplayName),
		AslanURL:    configbase.SystemAddress(),
		PipeName:    workflowArgs.Name,
		DisplayName: getDisplayName(workflowArgs),
		ProductName: workflowArgs.Project,
		PipeType:    config.WorkflowTypeV4,
		TaskID:      taskID,
	})
}

func (s *Service) UpdateGitCheckForWorkflowV4(workflowArgs *models.WorkflowV4, taskID int64, log *zap.SugaredLogger) error {
	hook := workflowArgs.HookPayload

	if hook == nil || !hook.IsPr {
		return nil
	}

	ghApp, err := github.GetGithubAppClientByOwner(hook.Owner)
	if err != nil {
		log.Errorf("getGithubAppClient failed, err:%v", err)
		return e.ErrGithubUpdateStatus.AddErr(err)
	}
	if ghApp != nil {
		log.Info("GitHub App found, start to update check-run")
		if hook.CheckRunID == 0 {
			log.Warn("No check-run ID found, skip")
			return nil
		}

		opt := &github.GitCheck{
			Owner:  hook.Owner,
			Repo:   hook.Repo,
			Branch: hook.Ref,
			Ref:    hook.Ref,
			IsPr:   hook.IsPr,

			AslanURL:    configbase.SystemAddress(),
			PipeName:    workflowArgs.Name,
			DisplayName: getDisplayName(workflowArgs),
			PipeType:    config.WorkflowTypeV4,
			ProductName: workflowArgs.Project,
			TaskID:      taskID,
		}

		return ghApp.UpdateGitCheck(hook.CheckRunID, opt)
	}

	log.Info("Start to update GitHub status to running")
	ch, err := systemconfig.New().GetCodeHost(hook.CodehostID)
	if err != nil {
		log.Errorf("Failed to get codeHost, err:%v", err)
		return e.ErrGithubUpdateStatus.AddErr(err)
	}
	gc := github.NewClient(ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)

	return gc.UpdateCheckStatus(&github.StatusOptions{
		Owner:       hook.Owner,
		Repo:        hook.Repo,
		Ref:         hook.Ref,
		State:       github.StatePending,
		Description: fmt.Sprintf("Workflow [%s] is running.", workflowArgs.DisplayName),
		AslanURL:    configbase.SystemAddress(),
		PipeName:    workflowArgs.Name,
		DisplayName: getDisplayName(workflowArgs),
		PipeType:    config.WorkflowTypeV4,
		ProductName: workflowArgs.Project,
		TaskID:      taskID,
	})
}

func (s *Service) CompleteGitCheckForWorkflowV4(workflowArgs *models.WorkflowV4, taskID int64, status config.Status, log *zap.SugaredLogger) error {
	hook := workflowArgs.HookPayload

	if hook == nil || !hook.IsPr {
		return nil
	}

	ghApp, err := github.GetGithubAppClientByOwner(hook.Owner)
	if err != nil {
		log.Errorf("getGithubAppClient failed, err:%v", err)
		return e.ErrGithubUpdateStatus.AddErr(err)
	}
	if ghApp != nil {
		log.Infof("GitHub App found, start to complete check-run")
		if hook.CheckRunID == 0 {
			log.Warn("No check-run ID found, skip")
			return nil
		}

		opt := &github.GitCheck{
			Owner:  hook.Owner,
			Repo:   hook.Repo,
			Branch: hook.Ref,
			Ref:    hook.Ref,
			IsPr:   hook.IsPr,

			AslanURL:    configbase.SystemAddress(),
			PipeName:    workflowArgs.Name,
			DisplayName: getDisplayName(workflowArgs),
			PipeType:    config.WorkflowTypeV4,
			ProductName: workflowArgs.Project,
			TaskID:      taskID,
		}

		return ghApp.CompleteGitCheck(hook.CheckRunID, getCheckStatus(status), opt)
	}

	ciStatus := getCheckStatus(status)
	log.Infof("Start to update GitHub status to %s", ciStatus)
	ch, err := systemconfig.New().GetCodeHost(hook.CodehostID)
	if err != nil {
		log.Errorf("Failed to get codeHost, err:%v", err)
		return e.ErrGithubUpdateStatus.AddErr(err)
	}
	gc := github.NewClient(ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)

	return gc.UpdateCheckStatus(&github.StatusOptions{
		Owner:       hook.Owner,
		Repo:        hook.Repo,
		Ref:         hook.Ref,
		State:       getGitHubStatusFromCIStatus(ciStatus),
		Description: fmt.Sprintf("Workflow [%s] is %s.", workflowArgs.DisplayName, ciStatus),
		AslanURL:    configbase.SystemAddress(),
		PipeName:    workflowArgs.Name,
		DisplayName: getDisplayName(workflowArgs),
		PipeType:    config.WorkflowTypeV4,
		ProductName: workflowArgs.Project,
		TaskID:      taskID,
	})
}

func getCheckStatus(status config.Status) github.CIStatus {
	switch status {
	case config.StatusCreated, config.StatusRunning:
		return github.CIStatusNeutral
	case config.StatusTimeout:
		return github.CIStatusTimeout
	case config.StatusFailed:
		return github.CIStatusFailure
	case config.StatusPassed:
		return github.CIStatusSuccess
	case config.StatusSkipped:
		return github.CIStatusCancelled
	case config.StatusCancelled:
		return github.CIStatusCancelled
	case config.StatusReject:
		return github.CIStatusRejected
	default:
		return github.CIStatusError
	}
}

func getGitHubStatusFromCIStatus(status github.CIStatus) string {
	switch status {
	case github.CIStatusSuccess:
		return github.StateSuccess
	case github.CIStatusFailure:
		return github.StateFailure
	default:
		return github.StateError
	}
}

func getDisplayName(args *models.WorkflowV4) string {
	if args.DisplayName != "" {
		return args.DisplayName
	}
	return args.Name
}
