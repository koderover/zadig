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
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/microservice/jobexecutor/core/service/meta"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types/step"
	"github.com/koderover/zadig/pkg/util/fs"
)

const dockerExe = "docker"

type DockerBuildStep struct {
	spec           *step.StepDockerBuildSpec
	envs           []string
	secretEnvs     []string
	dirs           *meta.ExecutorWorkDirs
	logger         *zap.SugaredLogger
	infrastructure string
}

func NewDockerBuildStep(metaData *meta.JobMetaData, logger *zap.SugaredLogger) (*DockerBuildStep, error) {
	dockerBuildStep := &DockerBuildStep{dirs: metaData.Dirs, envs: metaData.Envs, secretEnvs: metaData.SecretEnvs, logger: logger, infrastructure: metaData.Infrastructure}
	spec := metaData.Step.Spec
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return dockerBuildStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &dockerBuildStep.spec); err != nil {
		return dockerBuildStep, fmt.Errorf("unmarshal spec %s to docker build spec failed", yamlBytes)
	}
	return dockerBuildStep, nil
}

func (s *DockerBuildStep) Run(ctx context.Context) error {
	start := time.Now()
	s.logger.Infof("Start docker build.")
	defer func() {
		s.logger.Infof("Docker build ended. Duration: %.2f seconds.", time.Since(start).Seconds())
	}()

	envMap := makeEnvMap(s.envs, s.secretEnvs)
	s.spec.WorkDir = replaceEnvWithValue(s.spec.WorkDir, envMap)
	s.spec.DockerFile = replaceEnvWithValue(s.spec.DockerFile, envMap)
	s.spec.BuildArgs = replaceEnvWithValue(s.spec.BuildArgs, envMap)

	if err := s.dockerLogin(); err != nil {
		return err
	}
	return s.runDockerBuild()
}

func (s DockerBuildStep) dockerLogin() error {
	if s.spec.DockerRegistry == nil {
		return nil
	}
	if s.spec.DockerRegistry.UserName != "" {
		Print(s.logger, fmt.Sprintf("Logining Docker Registry: %s.\n", s.spec.DockerRegistry.Host), s.infrastructure, s.dirs.JobLogPath)
		startTimeDockerLogin := time.Now()
		cmd := dockerLogin(s.spec.DockerRegistry.UserName, s.spec.DockerRegistry.Password, s.spec.DockerRegistry.Host)

		needPersistentLog := false
		var fileName string
		if s.infrastructure == setting.JobVMInfrastructure {
			fileName = s.dirs.JobLogPath
			needPersistentLog = true
		}

		wg := &sync.WaitGroup{}
		err := SetCmdStdout(cmd, fileName, s.infrastructure, s.secretEnvs, needPersistentLog, wg)
		if err != nil {
			s.logger.Errorf("set cmd stdout error: %v", err)
			return err
		}
		//var out bytes.Buffer
		//cmd.Stdout = &out
		//cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to login docker registry: %v", err)
		}
		wg.Wait()

		Print(s.logger, fmt.Sprintf("Login ended. Duration: %.2f seconds.\n", time.Since(startTimeDockerLogin).Seconds()), s.infrastructure, s.dirs.JobLogPath)
	}
	return nil
}

func (s *DockerBuildStep) runDockerBuild() error {
	if s.spec == nil {
		return nil
	}

	Print(s.logger, fmt.Sprintf("Preparing Dockerfile.\n"), s.infrastructure, s.dirs.JobLogPath)
	startTimePrepareDockerfile := time.Now()
	err := s.prepareDockerfile(s.spec.Source, s.spec.DockerTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to prepare dockerfile: %s", err)
	}
	Print(s.logger, fmt.Sprintf("Preparation ended. Duration: %.2f seconds.\n", time.Since(startTimePrepareDockerfile).Seconds()), s.infrastructure, s.dirs.JobLogPath)

	if s.spec.Proxy != nil {
		setProxy(s.spec)
	}

	Print(s.logger, fmt.Sprintf("Running Docker Build.\n"), s.infrastructure, s.dirs.JobLogPath)
	startTimeDockerBuild := time.Now()
	envs := s.envs
	for _, c := range s.dockerCommands() {
		//c.Stdout = os.Stdout
		//c.Stderr = os.Stderr
		needPersistentLog := false
		var fileName string
		if s.infrastructure == setting.JobVMInfrastructure {
			fileName = s.dirs.JobLogPath
			needPersistentLog = true
		}

		wg := &sync.WaitGroup{}
		err := SetCmdStdout(c, fileName, s.infrastructure, s.secretEnvs, needPersistentLog, wg)
		if err != nil {
			s.logger.Errorf("set cmd stdout error: %v", err)
			return err
		}

		c.Dir = s.dirs.Workspace
		c.Env = envs
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to run docker build: %s", err)
		}
		wg.Wait()
	}
	Print(s.logger, fmt.Sprintf("Docker build ended. Duration: %.2f seconds.\n", time.Since(startTimeDockerBuild).Seconds()), s.infrastructure, s.dirs.JobLogPath)

	return nil
}

func (s *DockerBuildStep) dockerCommands() []*exec.Cmd {
	cmds := make([]*exec.Cmd, 0)
	if s.spec.WorkDir == "" {
		s.spec.WorkDir = "."
	}

	if s.infrastructure == setting.JobVMInfrastructure {
		s.spec.Workspace = s.dirs.Workspace
		s.spec.Infrastructure = setting.JobVMInfrastructure
	}
	cmds = append(
		cmds,
		dockerBuildCmd(
			s.spec.GetDockerFile(),
			s.spec.ImageName,
			s.spec.WorkDir,
			s.spec.BuildArgs,
			s.spec.IgnoreCache,
		),
		dockerPush(s.spec.ImageName),
	)
	return cmds
}

func dockerBuildCmd(dockerfile, fullImage, ctx, buildArgs string, ignoreCache bool) *exec.Cmd {
	args := []string{"-c"}
	dockerCommand := "docker build --rm=true"
	if ignoreCache {
		dockerCommand += " --no-cache"
	}

	if buildArgs != "" {
		for _, val := range strings.Fields(buildArgs) {
			if val != "" {
				dockerCommand = dockerCommand + " " + val
			}
		}

	}
	dockerCommand = dockerCommand + " -t " + fullImage + " -f " + dockerfile + " " + ctx
	args = append(args, dockerCommand)
	return exec.Command("sh", args...)
}

func dockerPush(fullImage string) *exec.Cmd {
	args := []string{"-c"}
	dockerPushCommand := "docker push " + fullImage
	args = append(args, dockerPushCommand)
	return exec.Command("sh", args...)
}

func dockerLogin(user, password, registry string) *exec.Cmd {
	return exec.Command(
		dockerExe,
		"login",
		"-u", user,
		"-p", password,
		registry,
	)
}

func (s *DockerBuildStep) prepareDockerfile(dockerfileSource, dockerfileContent string) error {
	if dockerfileSource == setting.DockerfileSourceTemplate {
		reader := strings.NewReader(dockerfileContent)
		readCloser := io.NopCloser(reader)
		var path string
		if s.infrastructure == setting.JobVMInfrastructure {
			path = filepath.Join(s.dirs.Workspace, setting.ZadigDockerfilePath)
		} else {
			path = fmt.Sprintf("/%s", setting.ZadigDockerfilePath)
		}
		err := fs.SaveFile(readCloser, path)
		if err != nil {
			return err
		}
	}
	return nil
}

func setProxy(ctx *step.StepDockerBuildSpec) {
	if ctx.Proxy.EnableRepoProxy && ctx.Proxy.Type == "http" {
		if !strings.Contains(strings.ToLower(ctx.BuildArgs), "--build-arg http_proxy=") {
			ctx.BuildArgs = fmt.Sprintf("%s --build-arg http_proxy=%s", ctx.BuildArgs, ctx.Proxy.GetProxyURL())
		}
		if !strings.Contains(strings.ToLower(ctx.BuildArgs), "--build-arg https_proxy=") {
			ctx.BuildArgs = fmt.Sprintf("%s --build-arg https_proxy=%s", ctx.BuildArgs, ctx.Proxy.GetProxyURL())
		}
	}
}
