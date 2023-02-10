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
	path := fmt.Sprintf("/zadig/debug/breakpoint_%s", s.Type)
	_, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warnf("debug step unexpected stat error: %v", err)
		}
		return nil
	}
	// This is to record that the debug step beginning and finished
	err = os.WriteFile(fmt.Sprintf("/zadig/debug/debug_%s", s.Type), nil, 0700)
	if err != nil {
		log.Errorf("debug step unexpected write file error: %v", err)
		return err
	}
	defer func() {
		err = os.WriteFile(fmt.Sprintf("/zadig/debug/debug_%s_done", s.Type), nil, 0700)
		if err != nil {
			log.Errorf("debug step unexpected write file error: %v", err)
		}
	}()

	log.Infof("debug step %s is waiting for breakpoint file remove", s.Type)
	for _, err := os.Stat(path); err == nil; {
		time.Sleep(time.Second)
	}
	log.Infof("debug step %s done", s.Type)
	return nil
}
