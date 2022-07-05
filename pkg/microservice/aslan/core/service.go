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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	commonconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	modeMongodb "github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/nsq"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/webhook"
	environmentservice "github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	labelMongodb "github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
	multiclusterservice "github.com/koderover/zadig/pkg/microservice/aslan/core/multicluster/service"
	systemrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/system/repository/mongodb"
	systemservice "github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflowcontroller"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"github.com/koderover/zadig/pkg/tool/rsa"
	"github.com/koderover/zadig/pkg/types"
)

const (
	webhookController = iota
)

type policyGetter interface {
	Policies() []*types.PolicyMeta
}

type Controller interface {
	Run(workers int, stopCh <-chan struct{})
}

func StartControllers(stopCh <-chan struct{}) {
	controllerWorkers := map[int]int{
		webhookController: 1,
	}
	controllers := map[int]Controller{
		webhookController: webhook.NewWebhookController(),
	}

	var wg sync.WaitGroup
	for name, c := range controllers {
		wg.Add(1)
		go func(name int, c Controller) {
			defer wg.Done()
			c.Run(controllerWorkers[name], stopCh)
		}(name, c)
	}

	wg.Wait()
}

func initRsaKey() {
	client, err := kubeclient.GetKubeClient(commonconfig.HubServerServiceAddress(), setting.LocalClusterID)
	if err != nil {
		log.DPanic(err)
	}
	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), setting.LocalClusterID)
	if err != nil {
		log.DPanic(err)
	}
	_, err = clientset.CoreV1().Secrets(commonconfig.Namespace()).Get(context.TODO(), setting.RSASecretName, metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			err, publicKey, privateKey := rsa.GetRsaKey()
			if err != nil {
				log.DPanic(err)
			}
			err = kube.CreateOrUpdateRSASecret(publicKey, privateKey, client)
			if err != nil {
				log.DPanic(err)
			}
		} else {
			log.DPanic(err)
		}
	}
}

func Start(ctx context.Context) {
	log.Init(&log.Config{
		Level:       commonconfig.LogLevel(),
		Filename:    commonconfig.LogFile(),
		SendToFile:  commonconfig.SendLogToFile(),
		Development: commonconfig.Mode() != setting.ReleaseMode,
	})

	initDatabase()

	initService()
	initDinD()

	systemservice.SetProxyConfig()

	workflowservice.InitPipelineController()
	workflowcontroller.InitWorkflowController()
	// 如果集群环境所属的项目不存在，则删除此集群环境
	environmentservice.CleanProducts()

	environmentservice.ResetProductsStatus()

	//Parse the workload dependencies configMap, PVC, ingress, secret
	go environmentservice.StartClusterInformer()

	go StartControllers(ctx.Done())

	go multiclusterservice.ClusterApplyUpgradeAgent()

	initRsaKey()
}

func Stop(ctx context.Context) {
	mongotool.Close(ctx)
}

func initService() {
	errors := new(multierror.Error)

	defer func() {
		if err := errors.ErrorOrNil(); err != nil {
			errMsg := fmt.Sprintf("New Aslan Service error: %v", err)
			log.Fatal(errMsg)
		}
	}()

	nsq.Init(config.PodName(), config.NsqLookupAddrs())

	if err := workflowservice.SubScribeNSQ(); err != nil {
		errors = multierror.Append(errors, err)
	}
}

func initDinD() {
	err := systemservice.SyncDinDForRegistries()
	if err != nil {
		log.Fatal(err)
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
		commonrepo.NewCallbackRequestColl(),
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
		commonrepo.NewDiffNoteColl(),
		commonrepo.NewDindCleanColl(),
		commonrepo.NewFavoriteColl(),
		commonrepo.NewGithubAppColl(),
		commonrepo.NewHelmRepoColl(),
		commonrepo.NewInstallColl(),
		commonrepo.NewItReportColl(),
		commonrepo.NewK8SClusterColl(),
		commonrepo.NewNotificationColl(),
		commonrepo.NewNotifyColl(),
		commonrepo.NewPipelineColl(),
		commonrepo.NewPrivateKeyColl(),
		commonrepo.NewProductColl(),
		commonrepo.NewProxyColl(),
		commonrepo.NewQueueColl(),
		commonrepo.NewRegistryNamespaceColl(),
		commonrepo.NewRenderSetColl(),
		commonrepo.NewS3StorageColl(),
		commonrepo.NewServiceColl(),
		commonrepo.NewStrategyColl(),
		commonrepo.NewStatsColl(),
		commonrepo.NewSubscriptionColl(),
		commonrepo.NewSystemSettingColl(),
		commonrepo.NewTaskColl(),
		commonrepo.NewTestTaskStatColl(),
		commonrepo.NewTestingColl(),
		commonrepo.NewWebHookColl(),
		commonrepo.NewWebHookUserColl(),
		commonrepo.NewWorkflowColl(),
		commonrepo.NewWorkflowStatColl(),
		commonrepo.NewWorkLoadsStatColl(),
		commonrepo.NewServicesInExternalEnvColl(),
		commonrepo.NewExternalLinkColl(),
		commonrepo.NewChartColl(),
		commonrepo.NewDockerfileTemplateColl(),
		commonrepo.NewProjectClusterRelationColl(),
		commonrepo.NewEnvResourceColl(),
		commonrepo.NewEnvSvcDependColl(),
		commonrepo.NewBuildTemplateColl(),
		commonrepo.NewScanningColl(),
		commonrepo.NewWorkflowV4Coll(),
		commonrepo.NewworkflowTaskv4Coll(),
		commonrepo.NewWorkflowQueueColl(),

		systemrepo.NewAnnouncementColl(),
		systemrepo.NewOperationLogColl(),
		labelMongodb.NewLabelColl(),
		labelMongodb.NewLabelBindingColl(),
		modeMongodb.NewCollaborationModeColl(),
		modeMongodb.NewCollaborationInstanceColl(),
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
	commonrepo.NewSystemSettingColl().InitSystemSettings()

	if err := commonrepo.NewS3StorageColl().InitData(); err != nil {
		log.Warnf("Failed to init S3 data: %s", err)
	}
}

type indexer interface {
	EnsureIndex(ctx context.Context) error
	GetCollectionName() string
}
