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

package jobcontroller

import (
	"context"
	"fmt"
	"strings"
	"time"

	zadigconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type FreestyleJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeclient  crClient.Client
	ack         func()
}

func NewFreestyleJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *FreestyleJobCtl {
	return &FreestyleJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
	}
}

func (c *FreestyleJobCtl) Run(ctx context.Context) {
	c.run(ctx)
	c.wait(ctx)
	c.complete(ctx)
}

func (c *FreestyleJobCtl) run(ctx context.Context) {
	// get kube client
	hubServerAddr := config.HubServerAddress()
	switch c.job.Properties.ClusterID {
	case setting.LocalClusterID:
		c.job.Properties.Namespace = zadigconfig.Namespace()
		c.kubeclient = krkubeclient.Client()
	default:
		c.job.Properties.Namespace = setting.AttachedClusterNamespace

		crClient, _, _, err := GetK8sClients(hubServerAddr, c.job.Properties.ClusterID)
		if err != nil {
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.job.EndTime = time.Now().Unix()
			return
		}
		c.kubeclient = crClient
	}

	// decide which docker host to use.
	// TODO: do not use code in warpdrive moudule, should move to a public place
	// dockerhosts := common.NewDockerHosts(hubServerAddr, c.logger)
	// dockerHost := dockerhosts.GetBestHost(common.ClusterID(c.job.Properties.ClusterID), "")

	// TODO: inject vars like task_id and etc, passing them into c.job.Properties.Args
	envNameVar := &commonmodels.KeyVal{Key: "ENV_NAME", Value: c.job.Properties.Namespace, IsCredential: false}
	c.job.Properties.Args = append(c.job.Properties.Args, envNameVar)

	jobCtxBytes, err := yaml.Marshal(BuildJobExcutorContext(c.job, c.workflowCtx))
	if err != nil {
		msg := fmt.Sprintf("cannot Jobexcutor.Context data: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}

	jobLabel := &JobLabel{
		WorkflowName: c.workflowCtx.WorkflowName,
		TaskID:       c.workflowCtx.TaskID,
		JobType:      string(c.job.JobType),
	}
	if err := ensureDeleteConfigMap(c.job.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
		c.logger.Error(err)
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return
	}

	if err := createJobConfigMap(
		c.job.Properties.Namespace, c.job.Name, jobLabel, string(jobCtxBytes), c.kubeclient); err != nil {
		msg := fmt.Sprintf("createJobConfigMap error: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}

	c.logger.Infof("succeed to create cm for job %s", c.job.Name)

	// TODO: do not use default image
	jobImage := "ccr.ccs.tencentyun.com/koderover-rc/job-excutor:guoyu-test1"
	// jobImage := getReaperImage(config.ReaperImage(), c.job.Properties.BuildOS)

	//Resource request default value is LOW
	job, err := buildJob(c.job.JobType, jobImage, c.job.Name, c.job.Properties.ClusterID, c.job.Properties.Namespace, c.job.Properties.ResourceRequest, setting.DefaultRequestSpec, c.job, c.workflowCtx, nil)
	if err != nil {
		msg := fmt.Sprintf("create job context error: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}

	job.Namespace = c.job.Properties.Namespace

	if err := ensureDeleteJob(c.job.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
		msg := fmt.Sprintf("delete job error: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}

	// 将集成到KodeRover的私有镜像仓库的访问权限设置到namespace中
	// if err := createOrUpdateRegistrySecrets(p.KubeNamespace, pipelineTask.ConfigPayload.RegistryID, p.Task.Registries, p.kubeClient); err != nil {
	// 	msg := fmt.Sprintf("create secret error: %v", err)
	// 	p.Log.Error(msg)
	// 	p.Task.TaskStatus = config.StatusFailed
	// 	p.Task.Error = msg
	// 	p.SetBuildStatusCompleted(config.StatusFailed)
	// 	return
	// }
	if err := updater.CreateJob(job, c.kubeclient); err != nil {
		msg := fmt.Sprintf("create job error: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}
	c.logger.Infof("succeed to create job %s", c.job.Name)
}

func (c *FreestyleJobCtl) wait(ctx context.Context) {
	status := waitJobEndWithFile(ctx, int(c.job.Properties.Timeout), c.job.Properties.Namespace, c.job.Name, c.kubeclient, c.logger)
	c.job.Status = status
}

func (c *FreestyleJobCtl) complete(ctx context.Context) {
	jobLabel := &JobLabel{
		WorkflowName: c.workflowCtx.WorkflowName,
		TaskID:       c.workflowCtx.TaskID,
		JobType:      string(c.job.JobType),
	}

	// 清理用户取消和超时的任务
	defer func() {
		if err := ensureDeleteJob(c.job.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
			c.logger.Error(err)
			c.job.Error = err.Error()
		}
		if err := ensureDeleteConfigMap(c.job.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
			c.logger.Error(err)
			c.job.Error = err.Error()
		}
	}()

	// err := saveContainerLog(pipelineTask, p.KubeNamespace, "", c.FileName, jobLabel, c.kubeclient)
	// if err != nil {
	// 	p.Log.Error(err)
	// 	p.Task.Error = err.Error()
	// 	return
	// }

	// p.Task.LogFile = p.FileName
}

func BuildJobExcutorContext(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx) *JobContext {
	var envVars, secretEnvVars []string
	for _, env := range job.Properties.Args {
		if env.IsCredential {
			secretEnvVars = append(secretEnvVars, strings.Join([]string{env.Key, env.Value}, "="))
			continue
		}
		envVars = append(envVars, strings.Join([]string{env.Key, env.Value}, "="))
	}

	workflowCtx.GlobalContextEach(func(k, v string) bool {
		envVars = append(envVars, strings.Join([]string{k, v}, "="))
		return true
	})

	return &JobContext{
		Name:         job.Name,
		Envs:         envVars,
		SecretEnvs:   secretEnvVars,
		WorkflowName: workflowCtx.WorkflowName,
		TaskID:       workflowCtx.TaskID,
		Steps:        job.Steps,
	}
}
