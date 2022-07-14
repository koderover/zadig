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

package scheduler

import (
	"fmt"
	stdlog "log"
	"os"
	"reflect"
	"time"

	"github.com/jasonlvhit/gocron"
	"github.com/nsqio/go-nsq"
	"github.com/rfyiamcool/cronlib"
	"go.uber.org/zap"

	newgoCron "github.com/go-co-op/gocron"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/cron/config"
	"github.com/koderover/zadig/pkg/microservice/cron/core/service"
	"github.com/koderover/zadig/pkg/microservice/cron/core/service/client"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/aslan"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

// CronClient ...
type CronClient struct {
	AslanCli                     *client.Client
	Schedulers                   map[string]*gocron.Scheduler
	SchedulerController          map[string]chan bool
	lastSchedulers               map[string][]*service.Schedule
	lastServiceSchedulers        map[string]*service.SvcRevision
	lastEnvSchedulerData         map[string]*service.ProductResp
	lastEnvResourceSchedulerData map[string]*service.EnvResource
	enabledMap                   map[string]bool
	lastPMProductRevisions       []*service.ProductRevision
	lastHelmProductRevisions     []*service.ProductRevision
	log                          *zap.SugaredLogger
}

type CronV3Client struct {
	Scheduler *newgoCron.Scheduler
	AslanCli  *aslan.Client
}

func NewCronV3() *CronV3Client {
	return &CronV3Client{
		Scheduler: newgoCron.NewScheduler(time.Local),
		AslanCli:  aslan.New(configbase.AslanServiceAddress()),
	}
}

func (c *CronV3Client) Start() {
	var lastConfig *aslan.CleanConfig

	c.Scheduler.Every(5).Seconds().Do(func() {
		// get the docker clean config
		config, err := c.AslanCli.GetDockerCleanConfig()
		if err != nil {
			log.Errorf("get config err :%s", err)
			return
		}

		if !reflect.DeepEqual(lastConfig, config) {
			lastConfig = config
			log.Infof("config changed to %v", config)
			if config.CronEnabled {
				c.Scheduler.RemoveByTag(string(types.CleanDockerTag))
				_, err = c.Scheduler.Cron(config.Cron).Tag(string(types.CleanDockerTag)).Do(func() {
					log.Infof("trigger aslan docker clean,reg: %v", config.Cron)
					// call docker clean
					if err := c.AslanCli.DockerClean(); err != nil {
						log.Errorf("fail to clean docker cache , err:%s", err)
					}
				})
				if err != nil {
					log.Errorf("fail to add docker_cache clean cron job:reg: %v,err:%s", config.Cron, err)
				}
			} else {
				log.Infof("remove docker_cache clean job , job tag: %v", types.CleanDockerTag)
				c.Scheduler.RemoveByTag(string(types.CleanDockerTag))

			}
		}
	})

	c.Scheduler.StartAsync()
}

const (
	CleanJobScheduler = "CleanJobScheduler"

	UpsertWorkflowScheduler = "UpsertWorkflowScheduler"

	UpsertTestScheduler = "UpsertTestScheduler"

	UpsertColliePipelineScheduler = "UpsertColliePipelineScheduler"

	CleanProductScheduler = "CleanProductScheduler"

	CleanCIResourcesScheduler = "CleanCIResourcesScheduler"

	InitStatScheduler = "InitStatScheduler"

	InitOperationStatScheduler = "InitOperationStatScheduler"

	InitPullSonarStatScheduler = "InitPullSonarStatScheduler"

	// SystemCapacityGC periodically triggers  garbage collection for system data based on its retention policy.
	SystemCapacityGC = "SystemCapacityGC"

	InitHealthCheckScheduler = "InitHealthCheckScheduler"

	InitHealthCheckPmHostScheduler = "InitHealthCheckPmHostScheduler"

	InitHelmEnvSyncValuesScheduler = "InitHelmEnvSyncValuesScheduler"

	EnvResourceSyncScheduler = "EnvResourceSyncScheduler"
)

// NewCronClient ...
// 服务初始化
func NewCronClient() *CronClient {
	nsqLookupAddrs := config.NsqLookupAddrs()

	aslanCli := client.NewAslanClient(fmt.Sprintf("%s/api", configbase.AslanServiceAddress()))
	//初始化nsq
	config := nsq.NewConfig()
	// 注意 WD_POD_NAME 必须使用 Downward API 配置环境变量
	config.UserAgent = "ASLAN_CRONJOB"
	config.MaxAttempts = 50
	config.LookupdPollInterval = 1 * time.Second

	//Cronjob Client
	cronjobClient, err := nsq.NewConsumer(setting.TopicCronjob, "cronjob", config)
	if err != nil {
		log.Fatalf("failed to init nsq consumer cronjob, error is %v", err)
	}
	cronjobClient.SetLogger(stdlog.New(os.Stdout, "nsq consumer:", 0), nsq.LogLevelError)

	cronjobScheduler := cronlib.New()
	cronjobScheduler.Start()

	cronjobHandler := NewCronjobHandler(aslanCli, cronjobScheduler)
	cronjobClient.AddConcurrentHandlers(cronjobHandler, 10)

	if err := cronjobClient.ConnectToNSQLookupds(nsqLookupAddrs); err != nil {
		errInfo := fmt.Sprintf("nsq consumer for cron job failed to start, the error is: %s", err)
		panic(errInfo)
	}

	return &CronClient{
		AslanCli:                     aslanCli,
		Schedulers:                   make(map[string]*gocron.Scheduler),
		lastSchedulers:               make(map[string][]*service.Schedule),
		lastServiceSchedulers:        make(map[string]*service.SvcRevision),
		lastEnvSchedulerData:         make(map[string]*service.ProductResp),
		lastEnvResourceSchedulerData: make(map[string]*service.EnvResource),
		SchedulerController:          make(map[string]chan bool),
		enabledMap:                   make(map[string]bool),
		log:                          log.SugaredLogger(),
	}
}

// 初始化轮询任务
func (c *CronClient) Init() {
	// 每天1点清理跑过的jobs
	c.InitCleanJobScheduler()
	// 每天2点 根据系统配额策略 清理系统过期数据
	c.InitSystemCapacityGCScheduler()
	// 定时任务触发
	c.InitJobScheduler()
	// 测试管理的定时任务触发
	c.InitTestScheduler()

	// 定时清理环境
	c.InitCleanProductScheduler()
	// clean collaboration instance resource every 5 minutes
	c.InitCleanCIResourcesScheduler()
	// 定时初始化构建数据
	c.InitBuildStatScheduler()
	// 定时器初始化话运营统计数据
	c.InitOperationStatScheduler()
	// 定时更新质效看板的统计数据
	c.InitPullSonarStatScheduler()
	// 定时初始化健康检查
	c.InitHealthCheckScheduler()
	// Timing probe host status
	c.InitHealthCheckPmHostScheduler()
	// sync values from remote for helm envs at regular intervals
	c.InitHelmEnvSyncValuesScheduler()
	// sync env resources from git at regular intervals
	c.InitEnvResourceSyncScheduler()
}

func (c *CronClient) InitCleanJobScheduler() {

	c.Schedulers[CleanJobScheduler] = gocron.NewScheduler()

	c.Schedulers[CleanJobScheduler].Every(1).Day().At("01:00").Do(c.AslanCli.TriggerCleanjobs, c.log)

	c.Schedulers[CleanJobScheduler].Start()
}

func (c *CronClient) InitCleanProductScheduler() {

	c.Schedulers[CleanProductScheduler] = gocron.NewScheduler()

	c.Schedulers[CleanProductScheduler].Every(5).Minutes().Do(c.AslanCli.TriggerCleanProducts, c.log)

	c.Schedulers[CleanProductScheduler].Start()
}

func (c *CronClient) InitCleanCIResourcesScheduler() {

	c.Schedulers[CleanCIResourcesScheduler] = gocron.NewScheduler()

	c.Schedulers[CleanCIResourcesScheduler].Every(5).Minutes().Do(c.AslanCli.TriggerCleanCIResources, c.log)

	c.Schedulers[CleanCIResourcesScheduler].Start()
}

func (c *CronClient) InitJobScheduler() {

	c.Schedulers[UpsertWorkflowScheduler] = gocron.NewScheduler()

	c.Schedulers[UpsertWorkflowScheduler].Every(1).Minutes().Do(c.UpsertWorkflowScheduler, c.log)

	c.Schedulers[UpsertWorkflowScheduler].Start()
}

func (c *CronClient) InitTestScheduler() {

	c.Schedulers[UpsertTestScheduler] = gocron.NewScheduler()

	c.Schedulers[UpsertTestScheduler].Every(1).Minutes().Do(c.UpsertTestScheduler, c.log)

	c.Schedulers[UpsertTestScheduler].Start()
}

func (c *CronClient) InitBuildStatScheduler() {
	c.Schedulers[InitStatScheduler] = gocron.NewScheduler()

	c.Schedulers[InitStatScheduler].Every(1).Day().At("01:00").Do(c.AslanCli.InitStatData, c.log)

	c.Schedulers[InitStatScheduler].Start()
}

func (c *CronClient) InitOperationStatScheduler() {

	c.Schedulers[InitOperationStatScheduler] = gocron.NewScheduler()

	c.Schedulers[InitOperationStatScheduler].Every(1).Hour().Do(c.AslanCli.InitOperationStatData, c.log)

	c.Schedulers[InitOperationStatScheduler].Start()
}

func (c *CronClient) InitPullSonarStatScheduler() {

	c.Schedulers[InitPullSonarStatScheduler] = gocron.NewScheduler()

	c.Schedulers[InitPullSonarStatScheduler].Every(10).Minutes().Do(c.AslanCli.InitPullSonarStatScheduler, c.log)

	c.Schedulers[InitPullSonarStatScheduler].Start()
}

func (c *CronClient) InitSystemCapacityGCScheduler() {

	c.Schedulers[SystemCapacityGC] = gocron.NewScheduler()

	c.Schedulers[SystemCapacityGC].Every(1).Day().At("02:00").Do(c.AslanCli.TriggerCleanCache, c.log)

	c.Schedulers[SystemCapacityGC].Start()
}

func (c *CronClient) InitHealthCheckScheduler() {

	c.Schedulers[InitHealthCheckScheduler] = gocron.NewScheduler()

	c.Schedulers[InitHealthCheckScheduler].Every(10).Seconds().Do(c.UpsertEnvServiceScheduler, c.log)

	c.Schedulers[InitHealthCheckScheduler].Start()
}

func (c *CronClient) InitHealthCheckPmHostScheduler() {

	c.Schedulers[InitHealthCheckPmHostScheduler] = gocron.NewScheduler()

	c.Schedulers[InitHealthCheckPmHostScheduler].Every(10).Seconds().Do(c.UpdatePmHostStatusScheduler, c.log)

	c.Schedulers[InitHealthCheckPmHostScheduler].Start()
}

func (c *CronClient) InitHelmEnvSyncValuesScheduler() {

	c.Schedulers[InitHelmEnvSyncValuesScheduler] = gocron.NewScheduler()

	c.Schedulers[InitHelmEnvSyncValuesScheduler].Every(20).Seconds().Do(c.UpsertEnvValueSyncScheduler, c.log)

	c.Schedulers[InitHelmEnvSyncValuesScheduler].Start()
}

func (c *CronClient) InitEnvResourceSyncScheduler() {
	c.Schedulers[EnvResourceSyncScheduler] = gocron.NewScheduler()

	c.Schedulers[EnvResourceSyncScheduler].Every(20).Seconds().Do(c.UpsertEnvResourceSyncScheduler, c.log)

	c.Schedulers[EnvResourceSyncScheduler].Start()
}
