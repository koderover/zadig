package step

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/koderover/zadig/pkg/types/step"
	"github.com/koderover/zadig/pkg/util"
	"gopkg.in/yaml.v3"
)

type ShellStep struct {
	spec       *step.StepShellSpec
	envs       []string
	secretEnvs []string
	workspace  string
}

func NewShellStep(spec interface{}, workspace string, envs, secretEnvs []string) (*ShellStep, error) {
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
	if len(s.spec.Scripts) == 0 {
		return nil
	}
	scripts := prepareScriptsEnv()

	scripts = append(scripts, s.spec.Scripts...)

	userScriptFile := "user_script.sh"
	if err := ioutil.WriteFile(filepath.Join(os.TempDir(), userScriptFile), []byte(strings.Join(scripts, "\n")), 0700); err != nil {
		return fmt.Errorf("write script file error: %v", err)
	}

	cmd := exec.Command("/bin/bash", filepath.Join(os.TempDir(), userScriptFile))
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
