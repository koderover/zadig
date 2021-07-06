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
	WorkflowPrefix = "workflow-"
	PipelinePrefix = "pipeline-"
	ColliePrefix   = "collie-"
	ServicePrefix  = "service-"

	taskTimeoutSecond = 10
)

type client struct {
	enabled bool
}

func NewClient() *client {
	return &client{
		enabled: false,
	}
}

type task struct {
	owner, repo, address, token, ref string
	from                             string
	add                              bool
	err                              error
	doneCh                           chan struct{}
}

func (c *client) AddWebHook(owner, repo, address, token, ref, from string) error {
	if !c.enabled {
		return nil
	}

	t := &task{
		owner:   owner,
		repo:    repo,
		address: address,
		token:   token,
		ref:     ref,
		from:    from,
		add:     true,
		doneCh:  make(chan struct{}),
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

func (c *client) RemoveWebHook(owner, repo, address, token, ref, from string) error {
	if !c.enabled {
		return nil
	}

	t := &task{
		owner:   owner,
		repo:    repo,
		address: address,
		token:   token,
		ref:     ref,
		from:    from,
		add:     false,
		doneCh:  make(chan struct{}),
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
