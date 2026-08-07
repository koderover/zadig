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

package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/helper"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/koderover/zadig/v2/pkg/util/fs"
)

const dockerExe = "docker"

type DockerBuildStep struct {
	spec       *step.StepDockerBuildSpec
	envs       []string
	secretEnvs []string
	logger     *log.JobLogger
	dirs       *types.AgentWorkDirs
}

func NewDockerBuildStep(spec interface{}, dirs *types.AgentWorkDirs, envs, secretEnvs []string, logger *log.JobLogger) (*DockerBuildStep, error) {
	dockerBuildStep := &DockerBuildStep{dirs: dirs, envs: envs, secretEnvs: secretEnvs, logger: logger}
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
		s.logger.Infof(fmt.Sprintf("Docker build ended. Duration: %.2f seconds.", time.Since(start).Seconds()))
	}()

	envMap := util.MakeEnvMap(s.envs, s.secretEnvs)
	s.spec.WorkDir = util.ReplaceEnvWithValue(s.spec.WorkDir, envMap)
	s.spec.DockerFile = util.ReplaceEnvWithValue(s.spec.DockerFile, envMap)
	s.spec.BuildArgs = util.ReplaceEnvWithValue(s.spec.BuildArgs, envMap)

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
		s.logger.Printf("Logining Docker Registry: %s.\n", s.spec.DockerRegistry.Host)
		startTimeDockerLogin := time.Now()
		cmd := dockerLogin(s.spec.DockerRegistry.UserName, s.spec.DockerRegistry.Password, s.spec.DockerRegistry.Host)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to login docker registry: %s %s", err, out.String())
		}

		s.logger.Printf("Login ended. Duration: %.2f seconds.\n", time.Since(startTimeDockerLogin).Seconds())
	}
	return nil
}

func (s *DockerBuildStep) runDockerBuild() error {
	if s.spec == nil {
		return nil
	}

	s.logger.Printf("Preparing Dockerfile.\n")
	startTimePrepareDockerfile := time.Now()
	err := prepareDockerfile(s.spec.Source, s.spec.DockerTemplateContent, s.dirs.Workspace)
	if err != nil {
		return fmt.Errorf("failed to prepare dockerfile: %s", err)
	}
	s.logger.Printf("Preparation ended. Duration: %.2f seconds.\n", time.Since(startTimePrepareDockerfile).Seconds())

	if s.spec.Proxy != nil {
		setProxy(s.spec)
	}

	s.logger.Printf("Runing Docker Build.\n")
	startTimeDockerBuild := time.Now()
	envs := s.envs
	for _, c := range s.dockerCommands() {
		c.Dir = s.dirs.Workspace
		c.Env = envs

		fileName := s.logger.GetLogfilePath()
		//如果文件不存在就创建文件，避免后面使用变量出错
		//util.WriteFile(fileName, []byte{}, 0700)

		needPersistentLog := true

		var wg sync.WaitGroup

		cmdStdoutReader, err := c.StdoutPipe()
		if err != nil {
			return err
		}

		// write script output to log file
		wg.Add(1)
		go func() {
			defer wg.Done()

			helper.HandleCmdOutput(cmdStdoutReader, needPersistentLog, fileName, s.secretEnvs, log.GetSimpleLogger())
		}()

		cmdStdErrReader, err := c.StderrPipe()
		if err != nil {
			return err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			helper.HandleCmdOutput(cmdStdErrReader, needPersistentLog, fileName, s.secretEnvs, log.GetSimpleLogger())
		}()

		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to run docker build: %s", err)
		}
		wg.Wait()
	}
	s.logger.Printf("Docker build ended. Duration: %.2f seconds.\n", time.Since(startTimeDockerBuild).Seconds())

	return nil
}

func (s *DockerBuildStep) dockerCommands() []*exec.Cmd {
	cmds := make([]*exec.Cmd, 0)

	buildCmd := dockerBuildCmd(
		s.spec.GetDockerFile(),
		s.spec.ImageName,
		s.spec.WorkDir,
		s.spec.BuildArgs,
		s.spec.IgnoreCache,
		s.spec.EnableBuildkit,
		s.spec.Platform,
	)

	if s.spec.EnableBuildkit {
		initBuildxCmd := dockerInitBuildxCmd(s.spec.Platform, s.spec.BuildKitImage)
		cmds = append(
			cmds,
			initBuildxCmd,
			buildCmd,
		)
	} else {
		pushCmd := dockerPush(s.spec.ImageName)

		cmds = append(
			cmds,
			buildCmd,
			pushCmd,
		)
	}
	return cmds
}

func dockerInitBuildxCmd(platform, buildKitImage string) *exec.Cmd {
	args := []string{"-c"}
	dockerInitBuildxCommand := fmt.Sprintf("docker buildx create --node=multiarch --use --platform %s --driver-opt=image=%s", platform, buildKitImage)
	args = append(args, dockerInitBuildxCommand)
	return exec.Command("sh", args...)
}

func dockerBuildCmd(dockerfile, fullImage, ctx, buildArgs string, ignoreCache, enableBuildkit bool, platform string) *exec.Cmd {
	args := []string{"-c"}
	dockerCommand := "docker build --rm=true"
	if enableBuildkit {
		dockerCommand = "docker buildx build --rm=true --push --platform " + platform
	}
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

// mac sudo echo "$CI_REGISTRY_PASSWORD" | docker login -u "$CI_REGISTRY_USER" "$CI_REGISTRY" --password-stdin
//func dockerLogin(user, password, registry string) *exec.Cmd {
//	return exec.Command(
//		dockerExe,
//		"login",
//		"-u", user,
//		"-p", password,
//		registry,
//	)
//}

func dockerLogin(user, password, registry string) *exec.Cmd {
	cmd := exec.Command("docker", "login", "-u", user, registry, "--password-stdin")
	cmd.Stdin = strings.NewReader(password)
	return cmd
}

func prepareDockerfile(dockerfileSource, dockerfileContent, workspace string) error {
	if dockerfileSource == setting.DockerfileSourceTemplate {
		reader := strings.NewReader(dockerfileContent)
		readCloser := io.NopCloser(reader)
		path := fmt.Sprintf("%s/%s", workspace, setting.ZadigDockerfilePath)
		err := fs.SaveFile(readCloser, path)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *DockerBuildStep) GetDockerFile() string {
	// if the source of the dockerfile is from template, we write our own dockerfile
	if s.spec.Source == setting.DockerfileSourceTemplate {
		return fmt.Sprintf("%s/%s", s.dirs.Workspace, setting.ZadigDockerfilePath)
	}
	if s.spec.DockerFile == "" {
		return "Dockerfile"
	}
	return s.spec.DockerFile
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
