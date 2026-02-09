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

package step

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

type ShellStep struct {
	spec       *step.StepShellSpec
	envs       []string
	secretEnvs []string
	workspace  string
	Paths      string
}

func NewShellStep(spec interface{}, workspace, paths string, envs, secretEnvs []string) (*ShellStep, error) {
	shellStep := &ShellStep{workspace: workspace, envs: envs, secretEnvs: secretEnvs}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return shellStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &shellStep.spec); err != nil {
		return shellStep, fmt.Errorf("unmarshal spec %s to shell spec failed", yamlBytes)
	}
	return shellStep, nil
}

func (s *ShellStep) Run(ctx context.Context) error {
	start := time.Now()
	log.Infof("Executing user script.")
	defer func() {
		log.Infof("Script Execution ended. Duration: %.2f seconds.", time.Since(start).Seconds())
	}()

	// This is to record that the shell step beginning and finished, for debug
	err := os.WriteFile("/zadig/debug/shell_step", nil, 0700)
	if err != nil {
		log.Warnf("Failed to record shell_step file, error: %v", err)
	}
	defer func() {
		err := os.WriteFile("/zadig/debug/shell_step_done", nil, 0700)
		if err != nil {
			log.Warnf("Failed to record shell_step_done file, error: %v", err)
		}
	}()

	if len(s.spec.Scripts) == 0 {
		return nil
	}
	scripts := []string{}
	if !s.spec.SkipPrepare {
		scripts = append(s.spec.Scripts[:1], append(prepareScriptsEnv(), s.spec.Scripts[1:]...)...)
	} else {
		scripts = append(scripts, s.spec.Scripts...)
	}

	userScriptFile := "user_script.sh"
	if err := ioutil.WriteFile(filepath.Join(os.TempDir(), userScriptFile), []byte(strings.Join(scripts, "\n")), 0700); err != nil {
		return fmt.Errorf("write script file error: %v", err)
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("/bin/bash", filepath.Join(os.TempDir(), userScriptFile))
	} else {
		cmd = exec.Command(filepath.Join(os.TempDir(), userScriptFile))
	}

	cmd.Dir = s.workspace
	cmd.Env = s.envs

	fileName := filepath.Join(os.TempDir(), "user_script.log")
	//如果文件不存在就创建文件，避免后面使用变量出错
	util.WriteFile(fileName, []byte{}, 0700)

	needPersistentLog := true

	var wg sync.WaitGroup

	cmdStdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		handleCmdOutput(cmdStdoutReader, needPersistentLog, fileName, s.secretEnvs)
	}()

	cmdStdErrReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		handleCmdOutput(cmdStdErrReader, needPersistentLog, fileName, s.secretEnvs)
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	wg.Wait()

	return cmd.Wait()
}
