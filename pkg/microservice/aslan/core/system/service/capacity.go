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
	"errors"
	"fmt"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/notify"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/poetry"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util"
)

const (
	defaultWorkflowMaxDays int = 365
	//logTag                     = "SysCap"
)

var defaultWorkflowTaskRetention = &commonmodels.CapacityStrategy{
	Target: commonmodels.WorkflowTaskRetention,
	Retention: &commonmodels.RetentionConfig{
		MaxDays: defaultWorkflowMaxDays,
	},
}

func UpdateSysCapStrategy(strategy *commonmodels.CapacityStrategy) error {
	if err := validateStrategy(strategy); err != nil {
		return err
	}

	err := commonrepo.NewStrategyColl().Upsert(strategy)
	if err != nil {
		return err
	}

	// 更新成功后，立即按照新的配置清理数据
	go handleWorkflowTaskRetentionCenter(strategy, false)

	return nil
}

func GetCapacityStrategy(target commonmodels.CapacityTarget) (*commonmodels.CapacityStrategy, error) {
	result, err := commonrepo.NewStrategyColl().GetByTarget(target)
	if err != nil && target == commonmodels.WorkflowTaskRetention {
		return defaultWorkflowTaskRetention, nil // Return default setup
	}
	return result, err
}

func HandleSystemGC(dryRun bool) error {
	// Find the strategy
	strategy, err := commonrepo.NewStrategyColl().GetByTarget(commonmodels.WorkflowTaskRetention)
	if err != nil {
		strategy = defaultWorkflowTaskRetention
	} else if err = validateStrategy(strategy); err != nil {
		return err
	}

	return handleWorkflowTaskRetentionCenter(strategy, dryRun)
}

func CleanCache() error {
	workflowMap := make(map[string]int)

	workflows, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{})
	if err != nil {
		log.Errorf("list workflows failed, err:%v", err)
		return err
	}
	for _, workflow := range workflows {
		// 注意s3的目录名有一个/后缀，处理时需要加一个/
		name := fmt.Sprintf("%s/", workflow.Name)
		if _, ok := workflowMap[name]; ok {
			continue
		}
		workflowMap[name] = 1
	}

	pipelines, err := commonrepo.NewPipelineColl().List(&commonrepo.PipelineListOption{})
	if err != nil {
		log.Errorf("list pipelines failed, err:%v", err)
		return err
	}
	for _, pipeline := range pipelines {
		name := fmt.Sprintf("%s/", pipeline.Name)
		if _, ok := workflowMap[name]; ok {
			continue
		}
		workflowMap[name] = 1
	}

	testings, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{TestType: setting.FunctionTest})
	if err != nil {
		log.Errorf("list testings failed, err:%v", err)
		return err
	}
	for _, testing := range testings {
		name := fmt.Sprintf("%s-%s/", testing.Name, "job")
		if _, ok := workflowMap[name]; ok {
			continue
		}
		workflowMap[name] = 1
	}

	s3Server := s3.FindInternalS3()
	client, err := s3tool.NewClient(s3Server.Endpoint, s3Server.Ak, s3Server.Sk, s3Server.Insecure, false)
	if err != nil {
		log.Errorf("Failed to create s3 client, error: %+v", err)
		return err
	}
	prefix := s3Server.GetObjectPath("")
	objects, err := client.ListFiles(s3Server.Bucket, prefix, false)
	if err != nil {
		log.Errorf("ListFiles failed, err:%v", err)
		return err
	}
	log.Infof("workflow count: %d, pipeline count: %d, testing count: %d", len(workflows), len(pipelines), len(testings))
	log.Infof("total paths in s3: %d, len(workflowMap): %d", len(objects), len(workflowMap))

	// 找出已被删除的工作流的缓存目录
	paths := make([]string, 0)
	for _, object := range objects {
		if _, ok := workflowMap[object]; ok {
			continue
		}
		log.Infof("ready to remove file or path:%s\n", object)
		paths = append(paths, object)
	}

	if err == nil {
		client.RemoveFiles(s3Server.Bucket, paths)
	}
	return nil
}

type MessageCtx struct {
	ReqID   string `bson:"req_id"                json:"req_id"`
	Title   string `bson:"title"                 json:"title"`   // 消息标题
	Content string `bson:"content"               json:"content"` // 消息内容
}

func sendSyscapNotify(handleErr error, totalCleanTasks *int) {
	content := &MessageCtx{
		ReqID: util.UUID(),
		Title: "清理历史工作流数据",
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	content.Content = fmt.Sprintf("清理时间: %s, 状态: 成功, 内容: 成功清理了%d条任务", now, *totalCleanTasks)
	if handleErr != nil {
		content.Content = fmt.Sprintf("清理时间: %s, 状态: 失败, 内容: %v", now, handleErr)
	}

	notifyInfo := &commonmodels.Notify{
		Type:       config.Message,
		Content:    content,
		CreateTime: time.Now().Unix(),
		IsRead:     false,
	}

	poetryClient := poetry.New(config.PoetryAPIServer(), config.PoetryAPIRootKey())
	users, _ := poetryClient.ListProductPermissionUsers("", "", log.SugaredLogger())
	for _, user := range users {
		notifyInfo.Receiver = user
		notify.NewNotifyClient().CreateNotify(user, notifyInfo)
	}
}

func handleWorkflowTaskRetentionCenter(strategy *commonmodels.CapacityStrategy, dryRun bool) error {
	var handleErr error
	var totalCleanTasks int

	// 向管理员发送通知，告知清理结果
	defer sendSyscapNotify(handleErr, &totalCleanTasks)

	// Figuring out starting option based on capacity strategy
	var option *commonrepo.ListAllTaskOption
	const batch = 100
	retention := strategy.Retention
	if retention.MaxDays > 0 {
		retentionTime := time.Now().AddDate(0, 0, -retention.MaxDays).Unix()
		option = &commonrepo.ListAllTaskOption{
			BeforeCreatTime: true,
			CreateTime:      retentionTime,
			Limit:           batch,
		}
		totalCleanTasks, handleErr = handleWorkflowTaskRetention(dryRun, batch, option)
		return handleErr
	}

	if retention.MaxItems > 0 {
		// 清理产品工作流任务数据
		workflows, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{})
		if err != nil {
			log.Errorf("list workflows failed, err:%v", err)
			handleErr = err
			return err
		}
		for _, workflow := range workflows {
			option = &commonrepo.ListAllTaskOption{
				ProductName:  workflow.ProductTmplName,
				PipelineName: workflow.Name,
				Type:         config.WorkflowType,
				Skip:         retention.MaxItems,
				Limit:        batch,
			}
			count, err := handleWorkflowTaskRetention(dryRun, batch, option)
			if err != nil {
				continue
			}
			totalCleanTasks += count
		}

		// 清理服务工作流任务数据
		pipelines, err := commonrepo.NewPipelineColl().List(&commonrepo.PipelineListOption{})
		if err != nil {
			log.Errorf("list pipelines failed, err:%v", err)
			handleErr = err
			return err
		}
		for _, pipeline := range pipelines {
			option = &commonrepo.ListAllTaskOption{
				ProductName:  pipeline.ProductName,
				PipelineName: pipeline.Name,
				Type:         config.SingleType,
				Skip:         retention.MaxItems,
				Limit:        batch,
			}
			count, err := handleWorkflowTaskRetention(dryRun, batch, option)
			if err != nil {
				continue
			}
			totalCleanTasks += count
		}

		// 清理测试任务数据
		testings, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{TestType: setting.FunctionTest})
		if err != nil {
			log.Errorf("list testings failed, err:%v", err)
			handleErr = err
			return err
		}
		for _, testing := range testings {
			option = &commonrepo.ListAllTaskOption{
				ProductName:  testing.ProductName,
				PipelineName: fmt.Sprintf("%s-%s", testing.Name, "job"),
				Type:         config.TestType,
				Skip:         retention.MaxItems,
				Limit:        batch,
			}
			count, err := handleWorkflowTaskRetention(dryRun, batch, option)
			if err != nil {
				continue
			}
			totalCleanTasks += count
		}
		return nil
	}

	return errors.New("no valid strategy for workflow task retention")
}

func handleWorkflowTaskRetention(dryRun bool, batch int, option *commonrepo.ListAllTaskOption) (int, error) {
	s3Server, _ := s3.FindDefaultS3()
	// Clean up in batches to prevent pressure of memory spike
	var removeIds []string
	for {
		staleTasks, err := commonrepo.NewTaskColl().ListTasks(option)
		if err != nil {
			return 0, err
		}
		if len(staleTasks) == 0 {
			break
		}
		ids := cleanStaleTasks(staleTasks, s3Server, dryRun)
		if ids != nil {
			removeIds = append(removeIds, ids...)
		}
		if len(staleTasks) < batch { // last batch
			break
		}
		option.Skip += batch
	}

	//if !dryRun {
	//	if err := s.taskRepo.DeleteByIds(removeIds); err != nil {
	//		return 0, err
	//	}
	//}

	log.Infof("%d stale workflow tasks will be cleaned up, productName:%s, workflowName:%s, type:%s", len(removeIds), option.ProductName, option.PipelineName, option.Type)
	return len(removeIds), nil
}

//
//func (s *Service) logInfo(format string, args ...interface{}) {
//	s.logger.Infof("[%v]: %v", logTag, fmt.Sprintf(format, args...))
//}
//
// cleanStaleTasks will mark stale tasks as deleted, and remove their relevant S3 files.
// returns:
//   []bson.ObjectId, task ides to be marked as deleted
func cleanStaleTasks(tasks []*task.Task, s3Server *s3.S3, dryRun bool) []string {
	ids := make([]string, len(tasks))
	paths := make([]string, len(tasks))
	for i, task := range tasks {
		ids[i] = task.ID.Hex()
		paths[i] = fmt.Sprintf("%s/%d/", task.PipelineName, task.TaskID)
	}
	forcedPathStyle := false
	if s3Server.Provider == setting.ProviderSourceSystemDefault {
		forcedPathStyle = true
	}
	s3client, err := s3tool.NewClient(s3Server.Endpoint, s3Server.Ak, s3Server.Sk, s3Server.Insecure, forcedPathStyle)
	if err == nil {
		go s3client.RemoveFiles(s3Server.Bucket, paths)
	}
	return ids
}

func validateStrategy(strategy *commonmodels.CapacityStrategy) error {
	if strategy.Target == commonmodels.WorkflowTaskRetention {
		retention := strategy.Retention
		if retention == nil {
			return errors.New("SysCap strategy: nil retention config for WorkflowTaskRetention")
		}
		if !(retention.MaxDays > 0 && retention.MaxItems == 0) &&
			!(retention.MaxDays == 0 && retention.MaxItems > 0) {
			return fmt.Errorf("SysCap strategy: max days or items value invalid, "+
				"can only set one positive value at a time. days: %v, items: %v",
				retention.MaxDays, retention.MaxItems)
		}
	} else {
		// Note: currently doesn't support other strategies yet.
		return fmt.Errorf("SysCap strategy target is invalid - passed in value: %v", strategy.Target)
	}
	return nil
}
