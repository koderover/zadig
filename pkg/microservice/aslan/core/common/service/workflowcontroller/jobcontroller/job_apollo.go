/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package jobcontroller

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	config2 "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/apollo"
)

type ApolloJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskApolloSpec
	ack         func()
}

func NewApolloJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *ApolloJobCtl {
	jobTaskSpec := &commonmodels.JobTaskApolloSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &ApolloJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *ApolloJobCtl) Clean(ctx context.Context) {}

func (c *ApolloJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	info, err := mongodb.NewConfigurationManagementColl().GetApolloByID(context.Background(), c.jobTaskSpec.ApolloID)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}
	link := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
		config2.SystemAddress(),
		c.workflowCtx.ProjectName,
		c.workflowCtx.WorkflowName,
		c.workflowCtx.TaskID,
		url.QueryEscape(c.workflowCtx.WorkflowDisplayName))

	releaseArgs := &apollo.ReleaseArgs{
		ReleaseTitle:   time.Now().Format("20060102150405") + "-zadig",
		ReleaseComment: fmt.Sprintf("工作流 %s\n详情: %s", c.workflowCtx.WorkflowDisplayName, link),
		ReleasedBy:     info.ApolloAuthConfig.User,
	}

	var fail bool
	client := apollo.NewClient(info.ServerAddress, info.Token)
	for _, namespace := range c.jobTaskSpec.NamespaceList {
		if namespace == nil {
			fail = true
			continue
		}
		namespace.Status = string(config.StatusRunning)
		namespace.Error = ""
		namespace.TargetResults = make([]*commonmodels.ApolloNamespaceTargetResult, 0)
		c.ack()

		if namespace.Action == "" {
			namespace.Action = commonmodels.ApolloActionUpdate
		}
		namespace.Type = strings.ToLower(strings.TrimSpace(namespace.Type))
		if namespace.Action == commonmodels.ApolloActionCreate && namespace.Type == "" {
			namespace.Type = apollo.FormatYAML
		}
		var err error
		switch namespace.Action {
		case commonmodels.ApolloActionCreate:
			err = c.createNamespace(client, namespace, info.ApolloAuthConfig.User, releaseArgs)
		case commonmodels.ApolloActionUpdate:
			err = updateAndReleaseNamespace(client, &namespace.ApolloNamespace, info.ApolloAuthConfig.User, releaseArgs)
		default:
			err = fmt.Errorf("unsupported apollo action: %s", namespace.Action)
		}
		if err != nil {
			fail = true
			namespace.Status = string(config.StatusFailed)
			namespace.Error = err.Error()
			c.ack()
			continue
		}
		namespace.Status = string(config.StatusPassed)
		c.ack()
	}
	if fail {
		logError(c.job, "some errors occurred in apollo job", c.logger)
		return
	}
	c.job.Status = config.StatusPassed
	c.ack()
}

func (c *ApolloJobCtl) createNamespace(client *apollo.Client, namespace *commonmodels.JobTaskApolloNamespace, user string, releaseArgs *apollo.ReleaseArgs) error {
	// Resolve and authorize every concrete target before creating the global AppNamespace.
	targets, err := client.ListAppEnvsAndClusters(namespace.AppID)
	if err != nil {
		return fmt.Errorf("list concrete namespace targets failed: %w", err)
	}
	if !c.jobTaskSpec.DisableConfigRange {
		allowedTargets := make(map[string]struct{}, len(c.jobTaskSpec.NamespaceListOption))
		for _, option := range c.jobTaskSpec.NamespaceListOption {
			if option != nil {
				allowedTargets[apolloTargetKey(option.AppID, option.Env, option.ClusterID)] = struct{}{}
			}
		}
		for _, env := range targets {
			if env == nil {
				continue
			}
			for _, cluster := range env.Clusters {
				key := apolloTargetKey(namespace.AppID, env.Env, cluster)
				if _, ok := allowedTargets[key]; !ok {
					return fmt.Errorf("apollo target [appID=%s, env=%s, cluster=%s] is not allowed to be created", namespace.AppID, env.Env, cluster)
				}
			}
		}
	}

	created, err := client.CreateAppNamespace(namespace.AppID, &apollo.CreateAppNamespaceArgs{
		Name:                strings.TrimSpace(namespace.Namespace),
		AppID:               namespace.AppID,
		Format:              namespace.Type,
		IsPublic:            false,
		Comment:             "created by Zadig workflow",
		DataChangeCreatedBy: user,
	})
	if err != nil {
		return fmt.Errorf("create app namespace failed: %w", err)
	}

	namespaceName := ""
	if created != nil {
		namespaceName = strings.TrimSpace(created.Name)
	}
	if namespaceName == "" {
		namespaceName = apollo.NormalizeNamespaceName(namespace.Namespace, namespace.Type)
	}
	namespace.Namespace = namespaceName

	for _, env := range targets {
		if env == nil {
			continue
		}
		for _, cluster := range env.Clusters {
			namespace.TargetResults = append(namespace.TargetResults, &commonmodels.ApolloNamespaceTargetResult{
				Env:       env.Env,
				ClusterID: cluster,
				Status:    string(config.StatusCreated),
			})
		}
	}
	c.ack()

	if len(namespace.TargetResults) == 0 {
		return errors.New("no concrete namespace target found")
	}

	partialFailure := false
	for _, target := range namespace.TargetResults {
		target.Status = string(config.StatusRunning)
		target.Error = ""
		c.ack()

		concreteNamespace, err := client.GetNamespace(namespace.AppID, target.Env, target.ClusterID, namespace.Namespace)
		if err != nil || concreteNamespace == nil {
			partialFailure = true
			target.Status = string(config.StatusFailed)
			if err != nil {
				target.Error = fmt.Sprintf("namespace not found after app namespace creation: %v", err)
			} else {
				target.Error = "namespace not found after app namespace creation"
			}
			c.ack()
			continue
		}

		concrete := namespace.ApolloNamespace
		concrete.Env = target.Env
		concrete.ClusterID = target.ClusterID
		if err := updateAndReleaseNamespace(client, &concrete, user, releaseArgs); err != nil {
			partialFailure = true
			target.Status = string(config.StatusFailed)
			target.Error = err.Error()
			c.ack()
			continue
		}

		target.Status = string(config.StatusPassed)
		c.ack()
	}
	if partialFailure {
		return errors.New("create namespace partially failed")
	}
	return nil
}

func apolloTargetKey(appID, env, cluster string) string {
	return fmt.Sprintf("%s++%s++%s", cluster, appID, env)
}

func updateAndReleaseNamespace(client *apollo.Client, namespace *commonmodels.ApolloNamespace, user string, releaseArgs *apollo.ReleaseArgs) error {
	for _, kv := range namespace.KeyValList {
		if kv == nil {
			return errors.New("update item failed: config item is nil")
		}
		kv.Key = apollo.NormalizeItemKey(namespace.Type, kv.Key)
		if err := client.UpdateKeyVal(namespace.AppID, namespace.Env, namespace.ClusterID, namespace.Namespace, kv.Key, kv.Val, user); err != nil {
			return fmt.Errorf("update item failed: %w", err)
		}
	}
	if err := client.Release(namespace.AppID, namespace.Env, namespace.ClusterID, namespace.Namespace, releaseArgs); err != nil {
		return fmt.Errorf("release failed: %w", err)
	}
	return nil
}

func (c *ApolloJobCtl) SaveInfo(ctx context.Context) error {
	return mongodb.NewJobInfoColl().Create(context.TODO(), &commonmodels.JobInfo{
		Type:                c.job.JobType,
		WorkflowName:        c.workflowCtx.WorkflowName,
		WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID:              c.workflowCtx.TaskID,
		ProductName:         c.workflowCtx.ProjectName,
		StartTime:           c.job.StartTime,
		EndTime:             c.job.EndTime,
		Duration:            c.job.EndTime - c.job.StartTime,
		Status:              string(c.job.Status),
	})
}
