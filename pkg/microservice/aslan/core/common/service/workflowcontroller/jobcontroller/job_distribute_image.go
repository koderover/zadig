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
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hashicorp/go-multierror"
	zadigconfig "github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/regclient/regclient"
	"github.com/regclient/regclient/config"
	"github.com/regclient/regclient/types/ref"
)

type DistributeImageCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	jobTaskSpec *commonmodels.JobTaskImageDistributeSpec
	ack         func()
}

func NewDistributeImageJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *DistributeImageCtl {
	jobTaskSpec := &commonmodels.JobTaskImageDistributeSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &DistributeImageCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *DistributeImageCtl) Clean(ctx context.Context) {}

func (c *DistributeImageCtl) Run(ctx context.Context) {
	if c.jobTaskSpec.SourceRegistry == nil || c.jobTaskSpec.TargetRegistry == nil {
		msg := "image registry infos are missing"
		logError(c.job, msg, c.logger)
		return
	}
	sourceHost := config.HostNewName(c.jobTaskSpec.SourceRegistry.RegAddr)
	sourceHost.User = c.jobTaskSpec.SourceRegistry.AccessKey
	sourceHost.Pass = c.jobTaskSpec.SourceRegistry.SecretKey

	targetHost := config.HostNewName(c.jobTaskSpec.TargetRegistry.RegAddr)
	targetHost.User = c.jobTaskSpec.TargetRegistry.AccessKey
	targetHost.Pass = c.jobTaskSpec.TargetRegistry.SecretKey
	hostsOpt := regclient.WithConfigHosts([]config.Host{*sourceHost, *targetHost})
	client := regclient.New(hostsOpt)

	errList := new(multierror.Error)
	wg := sync.WaitGroup{}
	for _, target := range c.jobTaskSpec.DistributeTarget {
		wg.Add(1)
		go func(target *commonmodels.DistributeTaskTarget) {
			defer wg.Done()
			if err := copyImage(target, client); err != nil {
				errList = multierror.Append(errList, err)
			}
		}(target)
	}
	wg.Wait()
	if err := errList.ErrorOrNil(); err != nil {
		msg := fmt.Sprintf("copy images error: %v", err)
		logError(c.job, msg, c.logger)
		return
	}
	c.job.Status = zadigconfig.StatusPassed
}

func copyImage(target *commonmodels.DistributeTaskTarget, client *regclient.RegClient) error {
	sourceRef, err := ref.New(target.SoureImage)
	if err != nil {
		errMsg := fmt.Sprintf("parse source image: %s error: %v", target.SoureImage, err)
		target.Status = zadigconfig.StatusFailed
		target.Error = errMsg
		return errors.New(errMsg)
	}
	targetRef, err := ref.New(target.TargetImage)
	if err != nil {
		errMsg := fmt.Sprintf("parse target image: %s error: %v", target.TargetImage, err)
		target.Status = zadigconfig.StatusFailed
		target.Error = errMsg
		return errors.New(errMsg)
	}
	if err := client.ImageCopy(context.Background(), sourceRef, targetRef); err != nil {
		errMsg := fmt.Sprintf("copy image failed: %v", err)
		target.Status = zadigconfig.StatusFailed
		target.Error = errMsg
		return errors.New(errMsg)
	}
	target.Status = zadigconfig.StatusPassed
	return nil
}
