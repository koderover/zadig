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

package step

import (
	"context"

	"github.com/koderover/zadig/pkg/tool/log"
)

type DebugStep struct {
	Type       string
	envs       []string
	secretEnvs []string
	workspace  string
}

func NewDebugStep(_type string, workspace string, envs, secretEnvs []string) (*DebugStep, error) {
	return &DebugStep{
		Type:       _type,
		envs:       envs,
		secretEnvs: secretEnvs,
		workspace:  workspace,
	}, nil
}

func (s *DebugStep) Run(ctx context.Context) error {
	log.Infof("%s debug step is running.", s.Type)

	return nil
}
