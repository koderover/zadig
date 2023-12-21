/*
Copyright 2023 The KodeRover Authors.

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

package script

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/helper"
)

type BatchFileStep struct {
	spec       *StepBatchFileSpec
	JobOutput  []string
	envs       []string
	secretEnvs []string
	dirs       *types.AgentWorkDirs
	Logger     *log.JobLogger
}

type StepBatchFileSpec struct {
	Scripts     []string `json:"scripts"                                 yaml:"scripts,omitempty"`
	Script      string   `json:"script"                                  yaml:"script"`
	SkipPrepare bool     `json:"skip_prepare"                            yaml:"skip_prepare"`
}

func NewBatchFileStep(jobOutput []string, spec interface{}, dirs *types.AgentWorkDirs, envs, secretEnvs []string, logger *log.JobLogger) (*BatchFileStep, error) {
	batchFileStep := &BatchFileStep{dirs: dirs, envs: envs, secretEnvs: secretEnvs, JobOutput: jobOutput}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return batchFileStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &batchFileStep.spec); err != nil {
		return batchFileStep, fmt.Errorf("unmarshal spec %s to script spec failed", yamlBytes)
	}
	batchFileStep.Logger = logger

	return batchFileStep, nil
}

func (s *BatchFileStep) Run(ctx context.Context) error {
	start := time.Now()
	s.Logger.Infof("Executing user script.")
	defer func() {
		s.Logger.Infof(fmt.Sprintf("Script Execution ended. Duration: %.2f seconds.", time.Since(start).Seconds()))
	}()

	userScriptFile, err := generateBatchFile(s.spec, s.dirs, s.JobOutput, s.Logger)
	if err != nil {
		return fmt.Errorf("generate script failed: %v", err)
	}
	cmd := exec.Command(userScriptFile)
	cmd.Dir = s.dirs.Workspace
	cmd.Env = s.envs

	fileName := s.Logger.GetLogfilePath()

	needPersistentLog := true

	var wg sync.WaitGroup

	cmdStdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	// write script output to log file
	wg.Add(1)
	go func() {
		defer wg.Done()

		helper.HandleCmdOutput(cmdStdoutReader, needPersistentLog, fileName, s.secretEnvs, log.GetSimpleLogger())
	}()

	cmdStdErrReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		helper.HandleCmdOutput(cmdStdErrReader, needPersistentLog, fileName, s.secretEnvs, log.GetSimpleLogger())
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	wg.Wait()

	return cmd.Wait()
}

func generateBatchFile(spec *StepBatchFileSpec, dirs *types.AgentWorkDirs, jobOutput []string, logger *log.JobLogger) (string, error) {
	if len(spec.Scripts) == 0 {
		return "", nil
	}
	scripts := []string{}
	scripts = append(scripts, spec.Scripts...)

	// add job output to script
	if len(jobOutput) > 0 {
		scripts = append(scripts, outputBatchFile(dirs.JobOutputsDir, jobOutput)...)
	}

	userScriptFile := config.GetUserBatchFileScriptFilePath(dirs.JobScriptDir)
	if err := ioutil.WriteFile(userScriptFile, []byte(strings.Join(scripts, "\n")), 0700); err != nil {
		return "", fmt.Errorf("write script file error: %v", err)
	}
	return userScriptFile, nil
}

// generate script to save outputs variable to file
func outputBatchFile(outputsDir string, outputs []string) []string {
	resp := []string{"@echo off"}
	for _, output := range outputs {
		resp = append(resp, fmt.Sprintf("echo %%%s%% > %s", output, path.Join(outputsDir, output)))
	}
	resp = append(resp, "@echo on")
	return resp
}
