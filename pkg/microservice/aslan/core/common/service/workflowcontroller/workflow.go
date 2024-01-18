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

package workflowcontroller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	config2 "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/instantmessage"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/scmnotify"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller/jobcontroller"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowstat"
	"github.com/koderover/zadig/v2/pkg/setting"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type workflowCtl struct {
	workflowTask       *commonmodels.WorkflowTask
	globalContextMutex *cache.RedisLock
	logger             *zap.SugaredLogger
	prefix             string
	ack                func()
}

func NewWorkflowController(workflowTask *commonmodels.WorkflowTask, logger *zap.SugaredLogger) *workflowCtl {
	ctl := &workflowCtl{
		workflowTask: workflowTask,
		logger:       logger,
		prefix:       fmt.Sprintf("workflowctl-%s-%d", workflowTask.WorkflowName, workflowTask.TaskID),
	}
	ctl.globalContextMutex = cache.NewRedisLock(fmt.Sprintf("%s-global-context", ctl.prefix))
	ctl.ack = ctl.updateWorkflowTask
	return ctl
}

func CancelWorkflowTask(userName, workflowName string, taskID int64, logger *zap.SugaredLogger) error {
	t, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		logger.Errorf("[%s] task: %s:%d not found", userName, workflowName, taskID)
		return err
	}

	// try to remove task from queue first.
	q := ConvertTaskToQueue(t)
	if err := Remove(q); err != nil {
		logger.Errorf("[%s] remove queue task: %s:%d error: %v", userName, workflowName, taskID, err)
	}

	if t.Status == config.StatusPassed {
		logger.Errorf("[%s] task: %s:%d is passed, cannot cancel", userName, workflowName, taskID)
		return fmt.Errorf("task: %s:%d is passed, cannot cancel", workflowName, taskID)
	}

	t.Status = config.StatusCancelled
	t.TaskRevoker = userName

	logger.Infof("[%s] CancelRunningTask %s:%d", userName, workflowName, taskID)

	if err := commonrepo.NewworkflowTaskv4Coll().Update(t.ID.Hex(), t); err != nil {
		logger.Errorf("[%s] update task: %s:%d error: %v", userName, workflowName, taskID, err)
		return err
	}

	// Updating the comment in the git repository, this will not cause the function to return error if this function call fails
	if err := scmnotify.NewService().UpdateWebhookCommentForWorkflowV4(t, logger); err != nil {
		log.Warnf("Failed to update comment for custom workflow %s, taskID: %d the error is: %s", t.WorkflowName, t.TaskID, err)
	}
	if err := scmnotify.NewService().CompleteGitCheckForWorkflowV4(t.WorkflowArgs, t.TaskID, t.Status, logger); err != nil {
		log.Warnf("Failed to update github check status for custom workflow %s, taskID: %d the error is: %s", t.WorkflowName, t.TaskID, err)
	}

	err = cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Publish(fmt.Sprintf("workflowctl-cancel-%s-%d", workflowName, taskID), "cancel")
	if err != nil {
		return fmt.Errorf("failed to cancel task: %s:%d, err: %s", workflowName, taskID, err)
	}
	return nil
}

func (c *workflowCtl) setWorkflowStatus(status config.Status) {
	c.workflowTask.Status = status
	c.ack()
}

func (c *workflowCtl) Run(ctx context.Context, concurrency int) {

	// TODO optimize me, this doesn't work when aslan running in multiple replicas
	addWorkflowTaskInMap(c.workflowTask.WorkflowName, c.workflowTask.TaskID, c.workflowTask, c.ack)
	defer removeWorkflowTaskInMap(c.workflowTask.WorkflowName, c.workflowTask.TaskID)

	c.workflowTask.Status = config.StatusRunning
	c.workflowTask.StartTime = time.Now().Unix()
	c.ack()
	c.logger.Infof("start workflow: %s,status: %s", c.workflowTask.WorkflowName, c.workflowTask.Status)
	defer func() {
		c.workflowTask.EndTime = time.Now().Unix()
		c.logger.Infof("finish workflow: %s,status: %s", c.workflowTask.WorkflowName, c.workflowTask.Status)
		c.ack()
		// clean share storage after workflow finished
		go c.CleanShareStorage()
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// sub cancel signal from redis
	cancelChan, closeFunc := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Subscribe(fmt.Sprintf("workflowctl-cancel-%s-%d", c.workflowTask.WorkflowName, c.workflowTask.TaskID))
	defer func() { _ = closeFunc() }()

	// receiving cancel signal from redis
	go func() {
		select {
		case <-cancelChan:
			cancel()
		case <-ctx.Done():
			return
		}
	}()

	workflowCtx := &commonmodels.WorkflowTaskCtx{
		WorkflowName:                c.workflowTask.WorkflowName,
		WorkflowDisplayName:         c.workflowTask.WorkflowDisplayName,
		ProjectName:                 c.workflowTask.ProjectName,
		TaskID:                      c.workflowTask.TaskID,
		WorkflowTaskCreatorUsername: c.workflowTask.TaskCreator,
		WorkflowTaskCreatorMobile:   c.workflowTask.TaskCreatorPhone,
		WorkflowTaskCreatorEmail:    c.workflowTask.TaskCreatorEmail,
		Workspace:                   "/workspace",
		DistDir:                     fmt.Sprintf("%s/%s/dist/%d", config.S3StoragePath(), c.workflowTask.WorkflowName, c.workflowTask.TaskID),
		DockerMountDir:              fmt.Sprintf("/tmp/%s/docker/%d", uuid.NewString(), time.Now().Unix()),
		ConfigMapMountDir:           fmt.Sprintf("/tmp/%s/cm/%d", uuid.NewString(), time.Now().Unix()),
		WorkflowKeyVals:             c.workflowTask.KeyVals,
		GlobalContextGetAll:         c.getGlobalContextAll,
		GlobalContextGet:            c.getGlobalContext,
		GlobalContextSet:            c.setGlobalContext,
		GlobalContextEach:           c.globalContextEach,
		ClusterIDAdd:                c.addCluterID,
		SetStatus:                   c.setWorkflowStatus,
		StartTime:                   time.Now(),
	}
	defer jobcontroller.CleanWorkflowJobs(ctx, c.workflowTask, workflowCtx, c.logger, c.ack)
	if err := scmnotify.NewService().UpdateWebhookCommentForWorkflowV4(c.workflowTask, c.logger); err != nil {
		log.Warnf("Failed to update comment for custom workflow %s, taskID: %d the error is: %s", c.workflowTask.WorkflowName, c.workflowTask.TaskID, err)
	}
	if err := scmnotify.NewService().UpdateGitCheckForWorkflowV4(c.workflowTask.WorkflowArgs, c.workflowTask.TaskID, c.logger); err != nil {
		log.Warnf("Failed to update github check status for custom workflow %s, taskID: %d the error is: %s", c.workflowTask.WorkflowName, c.workflowTask.TaskID, err)
	}
	RunStages(ctx, c.workflowTask.Stages, workflowCtx, concurrency, c.logger, c.ack)
	updateworkflowStatus(c.workflowTask)
	c.workflowTask.GlobalContext = c.getGlobalContextAllRaw()
}

func updateworkflowStatus(workflow *commonmodels.WorkflowTask) {
	statusMap := map[config.Status]int{
		config.StatusReject:    5,
		config.StatusCancelled: 4,
		config.StatusTimeout:   3,
		config.StatusFailed:    2,
		config.StatusPassed:    1,
		config.StatusSkipped:   0,
	}

	// 初始化workflowStatus为创建状态
	workflowStatus := config.StatusRunning

	stageStatus := make([]int, len(workflow.Stages))

	for i, j := range workflow.Stages {
		statusCode, ok := statusMap[j.Status]
		if !ok {
			statusCode = -1
		}
		stageStatus[i] = statusCode
	}
	var workflowStatusCode int
	for i, code := range stageStatus {
		if i == 0 || code > workflowStatusCode {
			workflowStatusCode = code
		}
	}

	for taskstatus, code := range statusMap {
		if workflowStatusCode == code {
			workflowStatus = taskstatus
			break
		}
	}
	workflow.Status = workflowStatus
}

func (c *workflowCtl) updateWorkflowTask() {
	taskInColl, err := commonrepo.NewworkflowTaskv4Coll().Find(c.workflowTask.WorkflowName, c.workflowTask.TaskID)
	if err != nil {
		c.logger.Errorf("find workflow task v4 %s failed,error: %v", c.workflowTask.WorkflowName, err)
		return
	}
	// 如果当前状态已经通过或者失败, 不处理新接受到的ACK
	if taskInColl.Status == config.StatusPassed || taskInColl.Status == config.StatusFailed || taskInColl.Status == config.StatusTimeout || taskInColl.Status == config.StatusReject {
		c.logger.Infof("%s:%d:%s task already done", c.workflowTask.WorkflowName, c.workflowTask.TaskID, taskInColl.Status)
		return
	}
	// 如果当前状态已经是取消状态, 一般为用户取消了任务, 此时任务在短暂时间内会继续运行一段时间,
	if taskInColl.Status == config.StatusCancelled {
		// Task终止状态可能为Pass, Fail, Cancel, Timeout
		// backend 会继续接受到ACK, 在这种情况下, 终止状态之外的ACK都无需处理，避免出现取消之后又被重置成运行态
		if c.workflowTask.Status != config.StatusFailed && c.workflowTask.Status != config.StatusPassed && c.workflowTask.Status != config.StatusCancelled && c.workflowTask.Status != config.StatusTimeout {
			c.logger.Infof("%s:%d task has been cancelled, ACK dropped", c.workflowTask.WorkflowName, c.workflowTask.TaskID)
			return
		}
	}
	if success := UpdateQueue(c.workflowTask); !success {
		c.logger.Errorf("%s:%d update t status error", c.workflowTask.WorkflowName, c.workflowTask.TaskID)
	}
	// TODO update workflow task
	if err := commonrepo.NewworkflowTaskv4Coll().Update(c.workflowTask.ID.Hex(), c.workflowTask); err != nil {
		c.logger.Errorf("update workflow task v4 failed,error: %v", err)
	}

	if c.workflowTask.Status == config.StatusPassed || c.workflowTask.Status == config.StatusFailed || c.workflowTask.Status == config.StatusTimeout || c.workflowTask.Status == config.StatusCancelled || c.workflowTask.Status == config.StatusReject {
		c.logger.Infof("%s:%d:%v task done", c.workflowTask.WorkflowName, c.workflowTask.TaskID, c.workflowTask.Status)
		if err := instantmessage.NewWeChatClient().SendWorkflowTaskNotifications(c.workflowTask); err != nil {
			c.logger.Errorf("send workflow task notification failed, error: %v", err)
		}
		q := ConvertTaskToQueue(c.workflowTask)
		if err := Remove(q); err != nil {
			c.logger.Errorf("remove queue task: %s:%d error: %v", c.workflowTask.WorkflowName, c.workflowTask.TaskID, err)
		}
		result, err := commonrepo.NewStrategyColl().GetByTarget(commonmodels.WorkflowTaskRetention)
		if err != nil {
			c.logger.Errorf("get workflow task retention strategy error: %s", err)
			result = commonmodels.DefaultWorkflowTaskRetention
		}
		if err = commonrepo.NewworkflowTaskv4Coll().ArchiveHistoryWorkflowTask(c.workflowTask.WorkflowName, result.Retention.MaxItems, result.Retention.MaxDays); err != nil {
			c.logger.Errorf("ArchiveHistoryWorkflowTask error: %v", err)
		}
		// Updating the comment in the git repository, this will not cause the function to return error if this function call fails
		if err := scmnotify.NewService().UpdateWebhookCommentForWorkflowV4(c.workflowTask, c.logger); err != nil {
			log.Warnf("Failed to update comment for custom workflow %s, taskID: %d the error is: %s", c.workflowTask.WorkflowName, c.workflowTask.TaskID, err)
		}
		if err := scmnotify.NewService().CompleteGitCheckForWorkflowV4(c.workflowTask.WorkflowArgs, c.workflowTask.TaskID, c.workflowTask.Status, c.logger); err != nil {
			log.Warnf("Failed to update github check status for custom workflow %s, taskID: %d the error is: %s", c.workflowTask.WorkflowName, c.workflowTask.TaskID, err)
		}
		if err := workflowstat.UpdateWorkflowStat(c.workflowTask.WorkflowName, string(config.WorkflowTypeV4), string(c.workflowTask.Status), c.workflowTask.ProjectName, c.workflowTask.EndTime-c.workflowTask.StartTime, c.workflowTask.IsRestart); err != nil {
			log.Warnf("Failed to update workflow stat for custom workflow %s, taskID: %d the error is: %s", c.workflowTask.WorkflowName, c.workflowTask.TaskID, err)
		}
	}
}

func (c *workflowCtl) CleanShareStorage() {
	clusterMap, err := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).HGetAllString(c.prefix + "-cluster")
	if err != nil {
		c.logger.Errorf("get cluster map error: %v", err)
	}

	for clusterID := range clusterMap {
		cleanJobName := fmt.Sprintf("clean-%s", rand.String(8))
		namespace := setting.AttachedClusterNamespace
		if clusterID == setting.LocalClusterID || clusterID == "" {
			namespace = config.Namespace()
		}
		kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
		if err != nil {
			c.logger.Errorf("can't init k8s client: %v", err)
			continue
		}
		kubeApiServer, err := kubeclient.GetKubeAPIReader(config.HubServerAddress(), clusterID)
		if err != nil {
			c.logger.Errorf("can't init k8s api reader: %v", err)
			continue
		}
		job, err := jobcontroller.BuildCleanJob(cleanJobName, clusterID, c.workflowTask.WorkflowName, c.workflowTask.TaskID)
		if err != nil {
			c.logger.Errorf("build clean job error: %v", err)
			continue
		}
		job.Namespace = namespace
		if err := updater.CreateJob(job, kubeClient); err != nil {
			c.logger.Errorf("create job error: %v", err)
			continue
		}
		defer func(client client.Client, name, namespace string) {
			if err := updater.DeleteJobAndWait(namespace, name, client); err != nil {
				c.logger.Errorf("delete job error: %v", err)
			}
		}(kubeClient, cleanJobName, namespace)
		status := jobcontroller.WaitPlainJobEnd(context.Background(), 10, namespace, cleanJobName, kubeClient, kubeApiServer, c.logger)
		c.logger.Infof("clean job %s finished, status: %s", cleanJobName, status)
	}

	cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Delete(c.prefix + "-cluster")
	cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Delete(c.prefix)
}

func (c *workflowCtl) addCluterID(clusterID string) {
	cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).HWrite(c.prefix+"-cluster", clusterID, "1", 0)
}

// mongo do not support dot in keys.
const (
	split = "@?"
)

func (c *workflowCtl) getGlobalContextAll() map[string]string {
	c.globalContextMutex.Lock()
	defer c.globalContextMutex.Unlock()

	res := make(map[string]string)
	contextMap, err := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).HGetAllString(c.prefix)
	if err != nil {
		log.Errorf("get global context %s error: %v", c.prefix, err)
		return res
	}

	for k, v := range contextMap {
		k = strings.Join(strings.Split(k, split), ".")
		res[k] = v
	}
	return res
}

func (c *workflowCtl) getGlobalContextAllRaw() map[string]string {
	c.globalContextMutex.Lock()
	defer c.globalContextMutex.Unlock()

	res := make(map[string]string)
	contextMap, err := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).HGetAllString(c.prefix)
	if err != nil {
		log.Errorf("get global context %s error: %v", c.prefix, err)
		return res
	}

	for k, v := range contextMap {
		res[k] = v
	}
	return res
}

func (c *workflowCtl) getGlobalContext(key string) (string, bool) {
	c.globalContextMutex.Lock()
	defer c.globalContextMutex.Unlock()

	v, err := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).HGetString(c.prefix, GetContextKey(key))
	existed := true
	if err != nil {
		existed = false
		if !errors.Is(err, redis.Nil) {
			log.Errorf("get global context %s error: %v", c.prefix, err)
		}
	}
	return v, existed
}

func (c *workflowCtl) setGlobalContext(key, value string) {
	c.globalContextMutex.Lock()
	defer c.globalContextMutex.Unlock()

	cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).HWrite(c.prefix, GetContextKey(key), value, 0)
}

func (c *workflowCtl) globalContextEach(f func(k, v string) bool) {
	c.globalContextMutex.Lock()
	defer c.globalContextMutex.Unlock()

	contextMap, err := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).HGetAllString(c.prefix)
	if err != nil {
		log.Errorf("get global context %s error: %v", c.prefix, err)
		return
	}

	for k, v := range contextMap {
		k = strings.Join(strings.Split(k, split), ".")
		if !f(k, v) {
			return
		}
	}
}

func GetContextKey(key string) string {
	return strings.Join(strings.Split(key, "."), split)
}
