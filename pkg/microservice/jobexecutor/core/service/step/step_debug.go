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
	"fmt"
	"os"
	"time"

	"github.com/koderover/zadig/pkg/microservice/jobexecutor/core/service/configmap"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

type DebugStep struct {
	Type       string
	envs       []string
	secretEnvs []string
	workspace  string
	updater    configmap.Updater
}

func NewDebugStep(_type string, workspace string, envs, secretEnvs []string, updater configmap.Updater) (*DebugStep, error) {
	return &DebugStep{
		Type:       _type,
		envs:       envs,
		secretEnvs: secretEnvs,
		workspace:  workspace,
		updater:    updater,
	}, nil
}

func (s *DebugStep) Run(ctx context.Context) (err error) {
	path := fmt.Sprintf("/zadig/debug/breakpoint_%s", s.Type)
	_, err = os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warnf("debug step unexpected stat error: %v", err)
		}
		return nil
	}
	// This is to record that the debug step beginning and finished
	cm, err := s.updater.Get()
	if err != nil {
		log.Errorf("debug step unexpected get configmap error: %v", err)
		return err
	}
	cm.Data[types.JobDebugStatusKey] = s.Type
	if s.updater.UpdateWithRetry(cm, 3, 3*time.Second) != nil {
		log.Errorf("debug step unexpected update configmap error: %v", err)
		return err
	}
	defer func() {
		cm, err = s.updater.Get()
		if err != nil {
			log.Errorf("debug step unexpected get configmap error: %v", err)
			return
		}
		cm.Data[types.JobDebugStatusKey] = types.JobDebugStatusNotIn
		if s.updater.UpdateWithRetry(cm, 3, 3*time.Second) != nil {
			log.Errorf("debug step unexpected update configmap error: %v", err)
		}
	}()

	log.Infof("Running debugger %s job, Use debugger console.", s.Type)
	for _, err := os.Stat(path); err == nil; {
		time.Sleep(time.Second)
		_, err = os.Stat(path)
	}
	log.Infof("debug step %s done", s.Type)
	return nil
}
