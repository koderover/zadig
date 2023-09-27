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
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/pkg/microservice/jobexecutor/core/service/meta"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/util"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/pkg/types/step"
)

type ShellStep struct {
	spec           *step.StepShellSpec
	envs           []string
	secretEnvs     []string
	Paths          string
	jobOutput      []string
	dirs           *meta.ExecutorWorkDirs
	logger         *zap.SugaredLogger
	infrastructure string
}

func NewShellStep(metaData *meta.JobMetaData, logger *zap.SugaredLogger) (*ShellStep, error) {
	shellStep := &ShellStep{dirs: metaData.Dirs, envs: metaData.Envs, secretEnvs: metaData.SecretEnvs, infrastructure: metaData.Infrastructure, logger: logger}
	spec := metaData.Step.Spec
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
	s.logger.Infof("Executing user script.")
	defer func() {
		s.logger.Infof("Script Execution ended. Duration: %.2f seconds.", time.Since(start).Seconds())
	}()

	if s.infrastructure != setting.JobVMInfrastructure {
		// This is to record that the shell step beginning and finished, for debug
		err := os.WriteFile("/zadig/debug/shell_step", nil, 0700)
		if err != nil {
			s.logger.Warnf("Failed to record shell_step file, error: %v", err)
		}
		defer func() {
			err := os.WriteFile("/zadig/debug/shell_step_done", nil, 0700)
			if err != nil {
				s.logger.Warnf("Failed to record shell_step_done file, error: %v", err)
			}
		}()
	}

	if len(s.spec.Scripts) == 0 {
		return nil
	}
	scriptFile, err := s.prepareScripts()
	if err != nil {
		return fmt.Errorf("prepare script file error: %v", err)
	}

	cmd := exec.Command("/bin/bash", scriptFile)
	cmd.Dir = s.dirs.Workspace
	cmd.Env = s.envs

	var fileName string
	if s.infrastructure != setting.JobVMInfrastructure {
		fileName = filepath.Join(os.TempDir(), "user_script.log")
		// if the file does not exist, create the file to avoid errors in the following use of the variable
		_ = util.WriteFile(fileName, []byte{}, 0700)
	} else {
		fileName = s.dirs.JobLogPath
	}

	wg := &sync.WaitGroup{}
	err = SetCmdStdout(cmd, fileName, s.secretEnvs, true, wg)
	if err != nil {
		s.logger.Errorf("set cmd stdout error: %v", err)
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	wg.Wait()

	return cmd.Wait()
}

func (s *ShellStep) prepareScripts() (string, error) {
	var filePath string
	scripts := []string{}
	if s.infrastructure != setting.JobVMInfrastructure {
		if !s.spec.SkipPrepare {
			scripts = prepareScriptsEnv()
		}
		scripts = append(scripts, s.spec.Scripts...)

		userScriptFile := "user_script.sh"
		if err := ioutil.WriteFile(filepath.Join(os.TempDir(), userScriptFile), []byte(strings.Join(scripts, "\n")), 0700); err != nil {
			return "", fmt.Errorf("write script file error: %v", err)
		}
		filePath = filepath.Join(os.TempDir(), userScriptFile)
	} else {
		scripts = append(scripts, s.spec.Scripts...)

		// add job output to script
		if len(s.jobOutput) > 0 {
			outputScript := []string{"set +ex"}
			for _, output := range s.jobOutput {
				outputScript = append(outputScript, fmt.Sprintf("echo $%s > %s", output, path.Join(s.dirs.JobOutputsDir, output)))
			}
			scripts = append(scripts, outputScript...)
		}

		userScriptFile := config.GetUserScriptFilePath(s.dirs.JobScriptDir)
		if err := ioutil.WriteFile(userScriptFile, []byte(strings.Join(scripts, "\n")), 0700); err != nil {
			return "", fmt.Errorf("write script file error: %v", err)
		}
		filePath = userScriptFile
	}
	return filePath, nil
}
