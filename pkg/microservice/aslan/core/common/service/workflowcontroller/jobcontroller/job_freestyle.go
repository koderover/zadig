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
	"net/url"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/informers"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	zadigconfig "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	vmmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/vm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	vmmongodb "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/vm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller/stepcontroller"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/multicluster/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/dockerhost"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

const (
	DindServer              = "dind"
	KoderoverAgentNamespace = "koderover-agent"
)

type FreestyleJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeclient  crClient.Client
	informer    informers.SharedInformerFactory
	apiServer   crClient.Reader
	paths       *string
	jobTaskSpec *commonmodels.JobTaskFreestyleSpec
	ack         func()
}

func NewFreestyleJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *FreestyleJobCtl {
	paths := ""
	jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &FreestyleJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		paths:       &paths,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *FreestyleJobCtl) Clean(ctx context.Context) {}

func (c *FreestyleJobCtl) Run(ctx context.Context) {
	if err := c.prepare(ctx); err != nil {
		return
	}

	// check the job is k8s job or vm job
	if c.job.Infrastructure == setting.JobVMInfrastructure {
		var vmJobID string
		var err error
		if vmJobID, err = c.runVMJob(ctx); err != nil {
			return
		}
		c.vmJobWait(ctx, vmJobID)
		c.vmComplete(ctx, vmJobID)
	} else {
		if err := c.run(ctx); err != nil {
			return
		}
		c.wait(ctx)
		c.complete(ctx)
	}
}

func (c *FreestyleJobCtl) prepare(ctx context.Context) error {
	for _, env := range c.jobTaskSpec.Properties.Envs {
		if strings.HasPrefix(env.Value, "{{.job") && strings.HasSuffix(env.Value, "}}") {
			env.Value = ""
		}
	}
	// set default timeout
	if c.jobTaskSpec.Properties.Timeout <= 0 {
		c.jobTaskSpec.Properties.Timeout = 600
	}
	// set default resource
	if c.jobTaskSpec.Properties.ResourceRequest == setting.Request("") {
		c.jobTaskSpec.Properties.ResourceRequest = setting.MinRequest
	}
	// set default resource
	if c.jobTaskSpec.Properties.ClusterID == "" {
		c.jobTaskSpec.Properties.ClusterID = setting.LocalClusterID
	}
	// init step configration.
	if err := stepcontroller.PrepareSteps(ctx, c.workflowCtx, &c.jobTaskSpec.Properties.Paths, c.job.Key, c.jobTaskSpec.Steps, c.logger); err != nil {
		logError(c.job, err.Error(), c.logger)
		return err
	}
	c.ack()
	return nil
}

func (c *FreestyleJobCtl) run(ctx context.Context) error {
	// get kube client
	hubServerAddr := zadigconfig.HubServerServiceAddress()
	if c.jobTaskSpec.Properties.ClusterID == setting.LocalClusterID {
		c.jobTaskSpec.Properties.Namespace = zadigconfig.Namespace()
	} else {
		c.jobTaskSpec.Properties.Namespace = setting.AttachedClusterNamespace
	}

	crClient, _, apiServer, err := GetK8sClients(hubServerAddr, c.jobTaskSpec.Properties.ClusterID)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return err
	}
	c.kubeclient = crClient
	c.apiServer = apiServer

	// decide which docker host to use.
	// TODO: do not use code in warpdrive moudule, should move to a public place
	dockerhosts := dockerhost.NewDockerHosts(hubServerAddr, c.logger)
	c.jobTaskSpec.Properties.DockerHost = dockerhosts.GetBestHost(dockerhost.ClusterID(c.jobTaskSpec.Properties.ClusterID), fmt.Sprintf("%v", c.workflowCtx.TaskID))

	// not local cluster
	var (
		replaceDindServer = "." + DindServer
		dockerHost        = ""
	)

	if c.jobTaskSpec.Properties.ClusterID != "" && c.jobTaskSpec.Properties.ClusterID != setting.LocalClusterID {
		if strings.Contains(c.jobTaskSpec.Properties.DockerHost, config.Namespace()) {
			// replace namespace only
			dockerHost = strings.Replace(c.jobTaskSpec.Properties.DockerHost, config.Namespace(), KoderoverAgentNamespace, 1)
		} else {
			// add namespace
			dockerHost = strings.Replace(c.jobTaskSpec.Properties.DockerHost, replaceDindServer, replaceDindServer+"."+KoderoverAgentNamespace, 1)
		}
	} else if c.jobTaskSpec.Properties.ClusterID == "" || c.jobTaskSpec.Properties.ClusterID == setting.LocalClusterID {
		if !strings.Contains(c.jobTaskSpec.Properties.DockerHost, config.Namespace()) {
			// add namespace
			dockerHost = strings.Replace(c.jobTaskSpec.Properties.DockerHost, replaceDindServer, replaceDindServer+"."+config.Namespace(), 1)
		}
	}

	c.jobTaskSpec.Properties.DockerHost = dockerHost

	jobCtxBytes, err := yaml.Marshal(BuildJobExecutorContext(c.jobTaskSpec, c.job, c.workflowCtx, c.logger))
	if err != nil {
		msg := fmt.Sprintf("cannot Jobexcutor.Context data: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	jobLabel := &JobLabel{
		JobType: string(c.job.JobType),
		JobName: c.job.K8sJobName,
	}
	if err := ensureDeleteConfigMap(c.jobTaskSpec.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
		logError(c.job, err.Error(), c.logger)
		return err
	}

	if err := createJobConfigMap(
		c.jobTaskSpec.Properties.Namespace, c.job.K8sJobName, jobLabel, string(jobCtxBytes), c.kubeclient); err != nil {
		msg := fmt.Sprintf("createJobConfigMap error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	c.logger.Infof("succeed to create cm for job %s", c.job.K8sJobName)

	if len(c.jobTaskSpec.Properties.Storages) > 0 {
		for i, storage := range c.jobTaskSpec.Properties.Storages {
			if storage.ProvisionType == types.DynamicProvision {
				err = service.CreateDynamicPVC(c.jobTaskSpec.Properties.ClusterID, getStoragePVCName(c.job.K8sJobName, i), storage, c.logger)
				if err != nil {
					msg := fmt.Sprintf("create dynamic PVC error: %v", err)
					logError(c.job, msg, c.logger)
					return errors.New(msg)

				}

				c.logger.Infof("succeed to create dynamic PVC for job %s", c.job.K8sJobName)
			}
		}
	}

	jobImage := getBaseImage(c.jobTaskSpec.Properties.BuildOS, c.jobTaskSpec.Properties.ImageFrom)

	c.jobTaskSpec.Properties.Registries = getMatchedRegistries(jobImage, c.jobTaskSpec.Properties.Registries)
	//Resource request default value is LOW
	customAnnotation := make(map[string]string)
	customLabel := make(map[string]string)

	for _, lb := range c.jobTaskSpec.Properties.CustomLabels {
		customLabel[lb.Key] = lb.Value.(string)
	}
	for _, annotate := range c.jobTaskSpec.Properties.CustomAnnotations {
		customAnnotation[annotate.Key] = annotate.Value.(string)
	}

	job, err := buildJob(c.job.JobType, jobImage, c.job.K8sJobName, c.jobTaskSpec.Properties.ClusterID, c.jobTaskSpec.Properties.Namespace, c.jobTaskSpec.Properties.ResourceRequest, c.jobTaskSpec.Properties.ResReqSpec, c.job, c.jobTaskSpec, c.workflowCtx, customLabel, customAnnotation)
	if err != nil {
		msg := fmt.Sprintf("create job context error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	job.Namespace = c.jobTaskSpec.Properties.Namespace

	if err := ensureDeleteJob(c.jobTaskSpec.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
		msg := fmt.Sprintf("delete job error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	if err := createOrUpdateRegistrySecrets(c.jobTaskSpec.Properties.Namespace, c.jobTaskSpec.Properties.Registries, c.kubeclient); err != nil {
		msg := fmt.Sprintf("create secret error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	if err := updater.CreateJob(job, c.kubeclient); err != nil {
		msg := fmt.Sprintf("create job error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	// set informer when job and cm have been created
	informer, err := clientmanager.NewKubeClientManager().GetInformer(c.jobTaskSpec.Properties.ClusterID, c.jobTaskSpec.Properties.Namespace)
	if err != nil {
		return errors.Wrap(err, "get informer")
	}
	c.informer = informer
	c.logger.Infof("succeed to create job %s", c.job.K8sJobName)
	return nil
}

func (c *FreestyleJobCtl) runVMJob(ctx context.Context) (string, error) {
	jobCtxBytes, err := yaml.Marshal(BuildJobExecutorContext(c.jobTaskSpec, c.job, c.workflowCtx, c.logger))
	if err != nil {

		msg := fmt.Sprintf("cannot Jobexcutor.Context data: %v", err)
		logError(c.job, msg, c.logger)
		return "", errors.New(msg)
	}
	jobInfo := new(commonmodels.TaskJobInfo)
	if err := commonmodels.IToi(c.job.JobInfo, jobInfo); err != nil {
		return "", fmt.Errorf("convert job info to task job info error: %v", err)
	}

	vmJob := new(vmmodels.VMJob)
	if c.workflowCtx != nil {
		vmJob.ProjectName = c.workflowCtx.ProjectName
		vmJob.WorkflowName = c.workflowCtx.WorkflowName
		vmJob.TaskID = c.workflowCtx.TaskID
		vmJob.JobName = c.job.Name
		vmJob.JobDisplayName = c.job.DisplayName
		vmJob.JobKey = c.job.Key
		vmJob.JobType = c.job.JobType
		vmJob.JobOriginName = jobInfo.JobName
	}

	vmJob.JobCtx = string(jobCtxBytes)
	vmJob.VMLabels = c.job.VMLabels
	vmJob.Status = setting.VMJobStatusCreated

	if err := vmmongodb.NewVMJobColl().Create(vmJob); err != nil {
		msg := fmt.Sprintf("create vm job error: %v", err)
		logError(c.job, msg, c.logger)
		return "", errors.New(msg)
	}
	return vmJob.ID.Hex(), nil
}

func (c *FreestyleJobCtl) wait(ctx context.Context) {
	var err error
	taskTimeout := time.After(time.Duration(c.jobTaskSpec.Properties.Timeout) * time.Minute)
	c.job.Status, err = waitJobStart(ctx, c.jobTaskSpec.Properties.Namespace, c.job.K8sJobName, c.kubeclient, c.apiServer, taskTimeout, c.logger)
	if err != nil {
		c.job.Error = err.Error()
	}
	if c.job.Status == config.StatusRunning {
		c.ack()
	} else {
		return
	}
	c.job.Status, c.job.Error = waitJobEndByCheckingConfigMap(ctx, taskTimeout, c.jobTaskSpec.Properties.Namespace, c.job.K8sJobName, true, c.informer, c.job, c.ack, c.logger)
}

func (c *FreestyleJobCtl) vmJobWait(ctx context.Context, jobID string) {
	var err error
	timeout := time.After(time.Duration(c.jobTaskSpec.Properties.Timeout) * time.Minute)

	// check job whether start
	c.job.Status, err = waitVMJobStart(ctx, jobID, timeout, c.job, c.logger)
	if err != nil {
		c.job.Error = err.Error()
	}
	if c.job.Status == config.StatusRunning {
		c.ack()
	} else {
		return
	}

	c.job.Status, c.job.Error = waitVMJobEndByCheckStatus(ctx, jobID, timeout, c.job, c.ack, c.logger)

	switch c.job.Status {
	case config.StatusCancelled:
		err := vmmongodb.NewVMJobColl().UpdateStatus(jobID, string(config.StatusCancelled))
		if err != nil {
			c.logger.Errorf("update vm job status error: %v", err)
			c.job.Error = fmt.Errorf("update vm job status %s error: %v", string(config.ReleasePlanStatusCancel), err).Error()
		}
	case config.StatusTimeout:
		err := vmmongodb.NewVMJobColl().UpdateStatus(jobID, string(config.StatusTimeout))
		if err != nil {
			c.logger.Errorf("update vm job status error: %v", err)
			c.job.Error = fmt.Errorf("update vm job status %s error: %v", string(config.StatusTimeout), err).Error()
		}
	}
}

func (c *FreestyleJobCtl) complete(ctx context.Context) {
	jobLabel := &JobLabel{
		JobType: string(c.job.JobType),
		JobName: c.job.K8sJobName,
	}

	// 清理用户取消和超时的任务
	defer func() {
		go func() {
			if len(c.jobTaskSpec.Properties.Storages) > 0 {
				for _, storage := range c.jobTaskSpec.Properties.Storages {
					if storage.IsTemporary {
						if err := ensureDeletePVC(storage.PVC, c.jobTaskSpec.Properties.Namespace, storage, c.kubeclient); err != nil {
							c.logger.Error(err)
						}
					}
				}
			}
			if err := ensureDeleteJob(c.jobTaskSpec.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
				c.logger.Error(err)
			}
			if err := ensureDeleteConfigMap(c.jobTaskSpec.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
				c.logger.Error(err)
			}
		}()
	}()

	// get job outputs info from pod terminate message.
	if err := getJobOutputFromConfigMap(c.jobTaskSpec.Properties.Namespace, c.job.Name, c.job, c.workflowCtx, c.informer); err != nil {
		c.logger.Error(err)
		c.job.Status, c.job.Error = config.StatusFailed, errors.Wrap(err, "get job outputs").Error()
	}

	if err := saveContainerLog(c.jobTaskSpec.Properties.Namespace, c.jobTaskSpec.Properties.ClusterID, c.workflowCtx.WorkflowName, c.job.Name, c.workflowCtx.TaskID, jobLabel, c.kubeclient); err != nil {
		c.logger.Error(err)
		if c.job.Error == "" {
			c.job.Error = err.Error()
		}
		return
	}
	if err := stepcontroller.SummarizeSteps(ctx, c.workflowCtx, &c.jobTaskSpec.Properties.Paths, c.job.Key, c.jobTaskSpec.Steps, c.logger); err != nil {
		c.logger.Error(err)
		c.job.Error = err.Error()
		return
	}
}

func (c *FreestyleJobCtl) vmComplete(ctx context.Context, jobID string) {
	defer func() {
		go func() {
			if err := vmmongodb.NewVMJobColl().DeleteByID(jobID, string(c.job.Status)); err != nil {
				c.logger.Error(fmt.Errorf("delete vm job error: %v", err))
			}
		}()
	}()

	// get job outputs info from job db
	if err := getVMJobOutputFromJobDB(jobID, c.job.Name, c.job, c.workflowCtx); err != nil {
		c.logger.Error(fmt.Errorf("get job outputs from job db error: %v", err))
		c.job.Status, c.job.Error = config.StatusFailed, fmt.Errorf("get job outputs from job db error: %v", err).Error()
	}

	// summarize steps
	if err := stepcontroller.SummarizeSteps(ctx, c.workflowCtx, &c.jobTaskSpec.Properties.Paths, c.job.Key, c.jobTaskSpec.Steps, c.logger); err != nil {
		c.logger.Error(err)
		c.job.Error = err.Error()
		return
	}
}

func getVMJobOutputFromJobDB(jobID, jobName string, job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx) error {
	vmJob, err := vmmongodb.NewVMJobColl().FindByID(jobID)
	if err != nil {
		return err
	}
	if vmJob == nil {
		return errors.New("vm job not found")
	}
	outputs := vmJob.Outputs
	writeOutputs(outputs, job.Key, workflowCtx)

	return nil
}

func BuildJobExecutorContext(jobTaskSpec *commonmodels.JobTaskFreestyleSpec, job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger) *JobContext {
	var envVars, secretEnvVars []string
	for _, env := range jobTaskSpec.Properties.Envs {
		if env.IsCredential {
			secretEnvVars = append(secretEnvVars, strings.Join([]string{env.Key, env.GetValue()}, "="))
			continue
		}
		envVars = append(envVars, strings.Join([]string{env.Key, env.GetValue()}, "="))
	}

	outputs := []string{}
	for _, output := range job.Outputs {
		outputs = append(outputs, output.Name)
	}

	jobContext := &JobContext{
		Name:          job.Name,
		Key:           job.Key,
		OriginName:    job.OriginName,
		DisplayName:   job.DisplayName,
		Envs:          envVars,
		SecretEnvs:    secretEnvVars,
		WorkflowName:  workflowCtx.WorkflowName,
		Workspace:     workflowCtx.Workspace,
		TaskID:        workflowCtx.TaskID,
		Outputs:       outputs,
		Paths:         jobTaskSpec.Properties.Paths,
		Steps:         jobTaskSpec.Steps,
		ConfigMapName: job.K8sJobName,
	}

	if job.Infrastructure == setting.JobVMInfrastructure {
		jobContext.Cache = &JobCacheConfig{
			CacheEnable:  jobTaskSpec.Properties.CacheEnable,
			CacheDirType: jobTaskSpec.Properties.CacheDirType,
			CacheUserDir: jobTaskSpec.Properties.CacheUserDir,
		}
	}

	return jobContext
}

func (c *FreestyleJobCtl) SaveInfo(ctx context.Context) error {
	// save delivery artifact for archive step
	if c.job.Status == config.StatusPassed {
		for _, stepTask := range c.jobTaskSpec.Steps {
			if stepTask.StepType == config.StepArchive {
				yamlString, err := yaml.Marshal(stepTask.Spec)
				if err != nil {
					return fmt.Errorf("marshal archive spec error: %v", err)
				}
				archiveSpec := &step.StepArchiveSpec{}
				if err := yaml.Unmarshal(yamlString, &archiveSpec); err != nil {
					return fmt.Errorf("unmarshal archive spec error: %v", err)
				}

				for _, upload := range archiveSpec.UploadDetail {
					if !upload.IsFileArchive {
						continue
					}
					deliveryArtifact := new(commonmodels.DeliveryArtifact)
					deliveryArtifact.CreatedBy = c.workflowCtx.WorkflowTaskCreatorUsername
					deliveryArtifact.CreatedTime = time.Now().Unix()
					deliveryArtifact.Source = string(config.WorkflowTypeV4)
					deliveryArtifact.Name = upload.ServiceModule + "_" + upload.ServiceName
					// TODO(Ray) file类型的交付物名称存放在Image和ImageTag字段是不规范的，优化时需要考虑历史数据的兼容问题。
					deliveryArtifact.Image = upload.Name
					deliveryArtifact.ImageTag = upload.Name
					deliveryArtifact.Type = string(config.File)
					deliveryArtifact.PackageFileLocation = upload.PackageFileLocation
					deliveryArtifact.PackageStorageURI = archiveSpec.S3.Endpoint + "/" + archiveSpec.S3.Bucket
					err := mongodb.NewDeliveryArtifactColl().Insert(deliveryArtifact)
					if err != nil {
						return fmt.Errorf("archiveCtl AfterRun: insert delivery artifact error: %v", err)
					}

					deliveryActivity := new(commonmodels.DeliveryActivity)
					deliveryActivity.Type = setting.BuildType
					deliveryActivity.ArtifactID = deliveryArtifact.ID
					deliveryActivity.JobTaskName = upload.JobTaskName
					deliveryActivity.URL = fmt.Sprintf("/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", c.workflowCtx.ProjectName, c.workflowCtx.WorkflowName, c.workflowCtx.TaskID, url.QueryEscape(c.workflowCtx.WorkflowDisplayName))
					commits := make([]*commonmodels.ActivityCommit, 0)
					for _, repo := range archiveSpec.Repos {
						deliveryCommit := new(commonmodels.ActivityCommit)
						deliveryCommit.Address = repo.Address
						deliveryCommit.Source = repo.Source
						deliveryCommit.RepoOwner = repo.RepoOwner
						deliveryCommit.RepoName = repo.RepoName
						deliveryCommit.Branch = repo.Branch
						deliveryCommit.Tag = repo.Tag
						deliveryCommit.PR = repo.PR
						deliveryCommit.PRs = repo.PRs
						deliveryCommit.CommitID = repo.CommitID
						deliveryCommit.CommitMessage = repo.CommitMessage
						deliveryCommit.AuthorName = repo.AuthorName

						commits = append(commits, deliveryCommit)
					}
					deliveryActivity.Commits = commits

					deliveryActivity.CreatedBy = c.workflowCtx.WorkflowTaskCreatorUsername
					deliveryActivity.CreatedTime = time.Now().Unix()
					deliveryActivity.StartTime = c.workflowCtx.StartTime.Unix()
					deliveryActivity.EndTime = time.Now().Unix()

					err = mongodb.NewDeliveryActivityColl().Insert(deliveryActivity)
					if err != nil {
						return fmt.Errorf("archiveCtl AfterRun: build deliveryActivityColl insert err:%v", err)
					}
				}

				break
			}
		}
	}

	jobInfo := &commonmodels.JobInfo{
		Type:                c.job.JobType,
		WorkflowName:        c.workflowCtx.WorkflowName,
		WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID:              c.workflowCtx.TaskID,
		ProductName:         c.workflowCtx.ProjectName,
		StartTime:           c.job.StartTime,
		EndTime:             c.job.EndTime,
		Duration:            c.job.EndTime - c.job.StartTime,
		Status:              string(c.job.Status),
	}

	if c.job.JobType == string(config.JobZadigVMDeploy) {
		jobInfo.ServiceName = c.jobTaskSpec.Properties.ServiceName
		jobInfo.ServiceModule = c.jobTaskSpec.Properties.ServiceName
	}

	return mongodb.NewJobInfoColl().Create(context.TODO(), jobInfo)
}
