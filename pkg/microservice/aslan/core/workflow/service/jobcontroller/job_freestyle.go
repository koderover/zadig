package jobcontroller

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	zadigconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type FreestyleJobCtl struct {
	job           *commonmodels.JobTask
	workflowCtx   *commonmodels.WorkflowTaskCtx
	jobContext    *sync.Map
	globalContext *sync.Map
	logger        *zap.SugaredLogger
	ack           func()
}

func NewFreestyleJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, globalContext *sync.Map, ack func(), logger *zap.SugaredLogger) *FreestyleJobCtl {
	return &FreestyleJobCtl{
		job:           job,
		workflowCtx:   workflowCtx,
		globalContext: globalContext,
		logger:        logger,
		ack:           ack,
	}
}

func (c *FreestyleJobCtl) Run(ctx context.Context) {
	r := rand.Intn(10)
	time.Sleep(time.Duration(r) * time.Second)
	c.logger.Infof("job: %s sleep %d seccod", c.job.Name, r)
	c.job.Status = config.StatusPassed
}

func (c *FreestyleJobCtl) run(ctx context.Context) {

	// get kube client
	var crClient crClient.Client
	var err error
	hubServerAddr := config.HubServerAddress()
	switch c.job.Properties.ClusterID {
	case setting.LocalClusterID:
		c.job.Namepace = zadigconfig.Namespace()
	default:
		c.job.Namepace = setting.AttachedClusterNamespace

		crClient, _, _, err = GetK8sClients(hubServerAddr, c.job.Properties.ClusterID)
		if err != nil {
			c.job.Status = config.StatusFailed
			c.job.Error = err.Error()
			c.job.EndTime = time.Now().Unix()
			return
		}
	}

	// decide which docker host to use.
	// TODO: do not use code in warpdrive moudule, should move to a public place
	// dockerhosts := common.NewDockerHosts(hubServerAddr, c.logger)
	// dockerHost := dockerhosts.GetBestHost(common.ClusterID(c.job.Properties.ClusterID), "")

	// TODO: inject vars like task_id and etc, passing them into c.job.Properties.Args
	envNameVar := &commonmodels.KeyVal{Key: "ENV_NAME", Value: c.job.Namepace, IsCredential: false}
	c.job.Properties.Args = append(c.job.Properties.Args, envNameVar)

	jobCtxBytes, err := yaml.Marshal(BuildJobExcutorContext(c.job, c.workflowCtx, c.globalContext))
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
	if err := ensureDeleteConfigMap(c.job.Namepace, jobLabel, crClient); err != nil {
		c.logger.Error(err)
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return
	}

	if err := createJobConfigMap(
		c.job.Namepace, c.job.Name, jobLabel, string(jobCtxBytes), crClient); err != nil {
		msg := fmt.Sprintf("createJobConfigMap error: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}

	c.logger.Infof("succeed to create cm for build job %s", c.job.Name)

	jobImage := getReaperImage(config.ReaperImage(), c.job.Properties.BuildOS)

	//Resource request default value is LOW
	job, err := buildJob(c.job.JobType, jobImage, c.job.Name, c.job.Properties.ClusterID, c.job.Namepace, c.job.Properties.ResourceRequest, setting.DefaultRequestSpec, c.job, c.workflowCtx, nil)
	if err != nil {
		msg := fmt.Sprintf("create build job context error: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}

	job.Namespace = c.job.Namepace

	if err := ensureDeleteJob(c.job.Namepace, jobLabel, crClient); err != nil {
		msg := fmt.Sprintf("delete build job error: %v", err)
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
	if err := updater.CreateJob(job, crClient); err != nil {
		msg := fmt.Sprintf("create build job error: %v", err)
		c.logger.Error(msg)
		c.job.Status = config.StatusFailed
		c.job.Error = msg
		return
	}
	c.logger.Infof("succeed to create build job %s", c.job.Name)
}
