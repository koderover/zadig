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

package github

import (
	"context"

	"github.com/google/go-github/v35/github"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
)

const (
	StateError   = "error"
	StateFailure = "failure"
	StatePending = "pending"
	StateSuccess = "success"
)

type StatusOptions struct {
	Owner       string
	Repo        string
	Ref         string
	State       string
	Description string

	AslanURL    string
	PipeName    string
	DisplayName string
	ProductName string
	PipeType    config.PipelineType
	TaskID      int64
}

func (c *Client) UpdateCheckStatus(opt *StatusOptions) error {
	sc := setting.ProductName + "/" + opt.DisplayName
	_, err := c.CreateStatus(
		context.TODO(), opt.Owner, opt.Repo, opt.Ref,
		&github.RepoStatus{
			State:       &opt.State,
			Description: &opt.Description,
			TargetURL: github.String(GetTaskLink(
				opt.AslanURL,
				opt.ProductName,
				opt.PipeName,
				opt.DisplayName,
				opt.PipeType,
				opt.TaskID,
			)),
			Context: &sc,
		})
	return err
}
