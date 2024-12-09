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

package cmd

import (
	"fmt"
	"os/exec"
)

// Command ...
type Command struct {
	Cmd           *exec.Cmd
	BeforeRun     func(args ...interface{}) error
	BeforeRunArgs []interface{}
	AfterRun      func(args ...interface{}) error
	AfterRunArgs  []interface{}
	// DisableTrace display command args
	DisableTrace bool
	// IgnoreError ingore command run error
	IgnoreError bool
}

func (c *Command) Run() error {
	if c.BeforeRun != nil {
		if err := c.BeforeRun(c.BeforeRunArgs...); err != nil {
			if !c.IgnoreError {
				return fmt.Errorf("before execute error: %v", err)
			}
		}
	}

	err := c.Cmd.Run()
	if !c.IgnoreError && err != nil {
		return err
	}

	if c.AfterRun != nil {
		if afterErr := c.AfterRun(c.AfterRunArgs...); afterErr != nil {
			if !c.IgnoreError {
				return fmt.Errorf("after execute error: %v", afterErr)
			}
		}
	}

	return nil
}
