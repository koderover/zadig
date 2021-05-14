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

package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo/template"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/nsq"
	environmentservice "github.com/koderover/zadig/lib/microservice/aslan/core/environment/service"
	systemservice "github.com/koderover/zadig/lib/microservice/aslan/core/system/service"
	workflowservice "github.com/koderover/zadig/lib/microservice/aslan/core/workflow/service/workflow"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func Setup() {
	initDatabase()

	initService()

	systemservice.SetProxyConfig()

	workflowservice.InitPipelineController()
	// 如果集群环境所属的项目不存在，则删除此集群环境
	environmentservice.CleanProducts()

	environmentservice.ResetProductsStatus()
}

func TearDown() {
	mongotool.Close(nil)
}

func initService() {
	log := xlog.NewWith("Aslan Service")
	log.SetLoggerLevel(config.LogLevel())
	errors := new(multierror.Error)

	defer func() {
		if err := errors.ErrorOrNil(); err != nil {
			errMsg := fmt.Sprintf("New Aslan Service error: %v", err)
			log.Fatal(errMsg)
		}
	}()

	nsq.Init(config.PodName(), config.NsqLookupAddrs())

	if err := workflowservice.SubScribeNSQ(log); err != nil {
		errors = multierror.Append(errors, err)
	}
}

func initDatabase() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mongotool.Init(ctx, config.MongoURI())
	if err := mongotool.Ping(ctx); err != nil {
		panic(fmt.Errorf("failed to connect to mongo, error: %s", err))
	}

	idxCtx, idxCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer idxCancel()

	var wg sync.WaitGroup
	for _, r := range []indexer{
		template.NewProductColl(),
		commonrepo.NewBasicImageColl(),
		commonrepo.NewBuildColl(),
		commonrepo.NewBuildStatColl(),
		commonrepo.NewConfigColl(),
		commonrepo.NewCounterColl(),
		commonrepo.NewCronjobColl(),
		commonrepo.NewDeliveryActivityColl(),
		commonrepo.NewDeliveryArtifactColl(),
		commonrepo.NewDeliveryBuildColl(),
		commonrepo.NewDeliveryDeployColl(),
		commonrepo.NewDeliveryDistributeColl(),
		commonrepo.NewDeliverySecurityColl(),
		commonrepo.NewDeliveryTestColl(),
		commonrepo.NewDeliveryVersionColl(),
		commonrepo.NewDeployStatColl(),
		commonrepo.NewDiffNoteColl(),
		commonrepo.NewDindCleanColl(),
		commonrepo.NewFavoriteColl(),
		commonrepo.NewGithubAppColl(),
		commonrepo.NewInstallColl(),
		commonrepo.NewItReportColl(),
		commonrepo.NewK8SClusterColl(),
		commonrepo.NewNotificationColl(),
		commonrepo.NewNotifyColl(),
		commonrepo.NewOperationLogColl(),
		commonrepo.NewPipelineColl(),
		commonrepo.NewProductColl(),
		commonrepo.NewProxyColl(),
		commonrepo.NewQueueColl(),
		commonrepo.NewRegistryNamespaceColl(),
		commonrepo.NewRenderSetColl(),
		commonrepo.NewS3StorageColl(),
		commonrepo.NewServiceColl(),
		commonrepo.NewStatsColl(),
		commonrepo.NewSubscriptionColl(),
		commonrepo.NewTaskColl(),
		commonrepo.NewTestTaskStatColl(),
		commonrepo.NewTestingColl(),
		commonrepo.NewWebHookUserColl(),
		commonrepo.NewWorkflowColl(),
		commonrepo.NewWorkflowStatColl(),
	} {
		wg.Add(1)
		go func(r indexer) {
			defer wg.Done()
			if err := r.EnsureIndex(idxCtx); err != nil {
				panic(fmt.Errorf("failed to create index for %s, error: %s", r.GetCollectionName(), err))
			}
		}(r)
	}

	wg.Wait()

	// 初始化数据
	commonrepo.NewInstallColl().InitInstallData(systemservice.InitInstallMap())
	commonrepo.NewBasicImageColl().InitBasicImageData(systemservice.InitbasicImageInfos())
}

type indexer interface {
	EnsureIndex(ctx context.Context) error
	GetCollectionName() string
}
