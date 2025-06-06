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

package webhook

import (
	"fmt"
	"time"
)

const (
	WorkflowPrefix   = "workflow-"
	WorkflowV4Prefix = "workflowv4-"
	PipelinePrefix   = "pipeline-"
	ColliePrefix     = "collie-"
	ServicePrefix    = "service-"
	TestingPrefix    = "testing-"
	ScannerPrefix    = "scanning-"

	taskTimeoutSecond = 10
)

type client struct {
	enabled bool
}

func NewClient() *client {
	return &client{
		enabled: true,
	}
}

type task struct {
	ID                                                          int
	owner, namespace, repo, address, token, ref, ak, sk, region string
	from                                                        string
	add, enableProxy, isManual, disbaleSSL                      bool
	err                                                         error
	doneCh                                                      chan struct{}
}

type TaskOption struct {
	ID          int
	Name        string
	Owner       string
	Namespace   string
	Repo        string
	Address     string
	Token       string
	Ref         string
	From        string
	AK          string
	SK          string
	Region      string
	IsManual    bool
	EnableProxy bool
	DisableSSL  bool
}

func (c *client) AddWebHook(taskOption *TaskOption) error {
	if !c.enabled {
		return nil
	}

	t := &task{
		ID:          taskOption.ID,
		owner:       taskOption.Owner,
		namespace:   taskOption.Namespace,
		repo:        taskOption.Repo,
		address:     taskOption.Address,
		token:       taskOption.Token,
		ref:         getFullReference(taskOption.Name, taskOption.Ref),
		from:        taskOption.From,
		add:         true,
		enableProxy: taskOption.EnableProxy,
		disbaleSSL:  taskOption.DisableSSL,
		ak:          taskOption.AK,
		sk:          taskOption.SK,
		region:      taskOption.Region,
		isManual:    taskOption.IsManual,
		doneCh:      make(chan struct{}),
	}

	select {
	case webhookController().queue <- t:
	default:
		return fmt.Errorf("queue is full, please retry it later")
	}

	select {
	case <-t.doneCh:
	case <-time.After(taskTimeoutSecond * time.Second):
		t.err = fmt.Errorf("timed out waiting for the task")
	}

	return t.err
}

func (c *client) RemoveWebHook(taskOption *TaskOption) error {
	if !c.enabled {
		return nil
	}

	t := &task{
		ID:          taskOption.ID,
		owner:       taskOption.Owner,
		namespace:   taskOption.Namespace,
		repo:        taskOption.Repo,
		address:     taskOption.Address,
		token:       taskOption.Token,
		ref:         getFullReference(taskOption.Name, taskOption.Ref),
		from:        taskOption.From,
		add:         false,
		enableProxy: taskOption.EnableProxy,
		disbaleSSL:  taskOption.DisableSSL,
		ak:          taskOption.AK,
		sk:          taskOption.SK,
		region:      taskOption.Region,
		isManual:    taskOption.IsManual,
		doneCh:      make(chan struct{}),
	}

	select {
	case webhookController().queue <- t:
	default:
		return fmt.Errorf("queue is full, please retry it later")
	}

	select {
	case <-t.doneCh:
	case <-time.After(taskTimeoutSecond * time.Second):
		t.err = fmt.Errorf("timed out waiting for the task")
	}

	return t.err
}

func getFullReference(hookName, ref string) string {
	return fmt.Sprintf("%s-%s", ref, hookName)
}
