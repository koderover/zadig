/*
Copyright 2025 The KodeRover Authors.

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
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/apisix"
)

type ApisixJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskApisixSpec
	ack         func()
}

func NewApisixJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *ApisixJobCtl {
	jobTaskSpec := &commonmodels.JobTaskApisixSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &ApisixJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *ApisixJobCtl) Clean(ctx context.Context) {}

func (c *ApisixJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	client, err := c.getApisixClient()
	if err != nil {
		logError(c.job, fmt.Sprintf("failed to get APISIX client: %v", err), c.logger)
		return
	}

	for _, task := range c.jobTaskSpec.Tasks {
		task.Status = string(config.StatusRunning)
		c.ack()

		if err := c.executeTask(client, task); err != nil {
			task.Status = string(config.StatusFailed)
			task.Error = err.Error()
			logError(c.job, fmt.Sprintf("failed to execute APISIX task: %v", err), c.logger)
			return
		}

		task.Status = string(config.StatusPassed)
		c.ack()
	}

	c.job.Status = config.StatusPassed
}

func (c *ApisixJobCtl) getApisixClient() (*apisix.Client, error) {
	gateway, err := commonrepo.NewApiGatewayColl().GetByID(c.jobTaskSpec.ApisixID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API gateway configuration: %v", err)
	}

	return apisix.NewClient(gateway.Address, gateway.Token), nil
}

func (c *ApisixJobCtl) executeTask(client *apisix.Client, task *commonmodels.ApisixItemUpdateSpec) error {
	switch task.Type {
	case config.ApisixItemTypeRoute:
		return c.executeRouteTask(client, task)
	case config.ApisixItemTypeUpstream:
		return c.executeUpstreamTask(client, task)
	case config.ApisixItemTypeService:
		return c.executeServiceTask(client, task)
	case config.ApisixItemTypeProto:
		return c.executeProtoTask(client, task)
	default:
		return fmt.Errorf("unsupported APISIX item type: %s", task.Type)
	}
}

func (c *ApisixJobCtl) executeRouteTask(client *apisix.Client, task *commonmodels.ApisixItemUpdateSpec) error {
	route, err := convertToRoute(task.UserSpec)
	if err != nil {
		return fmt.Errorf("failed to convert spec to route: %v", err)
	}

	switch task.Action {
	case config.ApisixActionTypeCreate:
		resp, err := client.CreateRoute(route)
		if err != nil {
			return err
		}
		if resp.Value != nil {
			task.ItemID = resp.Value.ID
		}
		return nil

	case config.ApisixActionTypeUpdate:
		if route.ID == "" {
			return fmt.Errorf("route id is required for update operation")
		}
		task.ItemID = route.ID
		_, err = client.UpdateRoute(route.ID, route)
		return err

	case config.ApisixActionTypeDelete:
		if route.ID == "" {
			return fmt.Errorf("route id is required for delete operation")
		}
		task.ItemID = route.ID
		return client.DeleteRoute(route.ID)

	default:
		return fmt.Errorf("unsupported action type: %s", task.Action)
	}
}

func (c *ApisixJobCtl) executeUpstreamTask(client *apisix.Client, task *commonmodels.ApisixItemUpdateSpec) error {
	upstream, err := convertToUpstream(task.UserSpec)
	if err != nil {
		return fmt.Errorf("failed to convert spec to upstream: %v", err)
	}

	switch task.Action {
	case config.ApisixActionTypeCreate:
		resp, err := client.CreateUpstream(upstream)
		if err != nil {
			return err
		}
		if resp.Value != nil {
			task.ItemID = resp.Value.ID
		}
		return nil

	case config.ApisixActionTypeUpdate:
		if upstream.ID == "" {
			return fmt.Errorf("upstream id is required for update operation")
		}
		task.ItemID = upstream.ID
		_, err = client.UpdateUpstream(upstream.ID, upstream)
		return err

	case config.ApisixActionTypeDelete:
		if upstream.ID == "" {
			return fmt.Errorf("upstream id is required for delete operation")
		}
		task.ItemID = upstream.ID
		return client.DeleteUpstream(upstream.ID)

	default:
		return fmt.Errorf("unsupported action type: %s", task.Action)
	}
}

func (c *ApisixJobCtl) executeServiceTask(client *apisix.Client, task *commonmodels.ApisixItemUpdateSpec) error {
	service, err := convertToService(task.UserSpec)
	if err != nil {
		return fmt.Errorf("failed to convert spec to service: %v", err)
	}

	switch task.Action {
	case config.ApisixActionTypeCreate:
		resp, err := client.CreateService(service)
		if err != nil {
			return err
		}
		if resp.Value != nil {
			task.ItemID = resp.Value.ID
		}
		return nil

	case config.ApisixActionTypeUpdate:
		if service.ID == "" {
			return fmt.Errorf("service id is required for update operation")
		}
		task.ItemID = service.ID
		_, err = client.UpdateService(service.ID, service)
		return err

	case config.ApisixActionTypeDelete:
		if service.ID == "" {
			return fmt.Errorf("service id is required for delete operation")
		}
		task.ItemID = service.ID
		return client.DeleteService(service.ID)

	default:
		return fmt.Errorf("unsupported action type: %s", task.Action)
	}
}

func (c *ApisixJobCtl) executeProtoTask(client *apisix.Client, task *commonmodels.ApisixItemUpdateSpec) error {
	proto, err := convertToProto(task.UserSpec)
	if err != nil {
		return fmt.Errorf("failed to convert spec to proto: %v", err)
	}

	switch task.Action {
	case config.ApisixActionTypeCreate:
		resp, err := client.CreateProto(proto)
		if err != nil {
			return err
		}
		if resp.Value != nil {
			task.ItemID = resp.Value.ID
		}
		return nil

	case config.ApisixActionTypeUpdate:
		if proto.ID == "" {
			return fmt.Errorf("proto id is required for update operation")
		}
		task.ItemID = proto.ID
		_, err = client.UpdateProto(proto.ID, proto)
		return err

	case config.ApisixActionTypeDelete:
		if proto.ID == "" {
			return fmt.Errorf("proto id is required for delete operation")
		}
		task.ItemID = proto.ID
		return client.DeleteProto(proto.ID)

	default:
		return fmt.Errorf("unsupported action type: %s", task.Action)
	}
}

// convertToRoute converts an interface{} to an apisix.Route
func convertToRoute(spec interface{}) (*apisix.Route, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal spec: %v", err)
	}

	route := &apisix.Route{}
	if err := json.Unmarshal(data, route); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to route: %v", err)
	}

	return route, nil
}

// convertToUpstream converts an interface{} to an apisix.Upstream
func convertToUpstream(spec interface{}) (*apisix.Upstream, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal spec: %v", err)
	}

	upstream := &apisix.Upstream{}
	if err := json.Unmarshal(data, upstream); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to upstream: %v", err)
	}

	return upstream, nil
}

// convertToService converts an interface{} to an apisix.Service
func convertToService(spec interface{}) (*apisix.Service, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal spec: %v", err)
	}

	service := &apisix.Service{}
	if err := json.Unmarshal(data, service); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to service: %v", err)
	}

	return service, nil
}

// convertToProto converts an interface{} to an apisix.Proto
func convertToProto(spec interface{}) (*apisix.Proto, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal spec: %v", err)
	}

	proto := &apisix.Proto{}
	if err := json.Unmarshal(data, proto); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to proto: %v", err)
	}

	return proto, nil
}

func (c *ApisixJobCtl) SaveInfo(ctx context.Context) error {
	return commonrepo.NewJobInfoColl().Create(context.TODO(), &commonmodels.JobInfo{
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
