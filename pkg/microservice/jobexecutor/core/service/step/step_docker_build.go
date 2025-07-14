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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/koderover/zadig/v2/pkg/util/fs"
)

const dockerExe = "docker"

type DockerBuildStep struct {
	spec       *step.StepDockerBuildSpec
	envs       []string
	secretEnvs []string
	workspace  string
}

func NewDockerBuildStep(spec interface{}, workspace string, envs, secretEnvs []string) (*DockerBuildStep, error) {
	dockerBuildStep := &DockerBuildStep{workspace: workspace, envs: envs, secretEnvs: secretEnvs}
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
	log.Infof("Start docker build.")

	envMap := util.MakeEnvMap(s.envs, s.secretEnvs)
	if image, ok := envMap["IMAGE"]; ok {
		s.spec.ImageName = image
	}

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
		log.Infof("Logging in Docker Registry: %s.", s.spec.DockerRegistry.Host)
		startTimeDockerLogin := time.Now()
		cmd := dockerLogin(s.spec.DockerRegistry.UserName, s.spec.DockerRegistry.Password, s.spec.DockerRegistry.Host)
		var out bytes.Buffer
		cmdOutReader, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}

		outScanner := bufio.NewScanner(cmdOutReader)
		go func() {
			for outScanner.Scan() {
				fmt.Printf("%s   %s\n", time.Now().Format(setting.WorkflowTimeFormat), outScanner.Text())
			}
		}()

		cmdErrReader, err := cmd.StderrPipe()
		if err != nil {
			return err
		}

		errScanner := bufio.NewScanner(cmdErrReader)
		go func() {
			for errScanner.Scan() {
				fmt.Printf("%s   %s\n", time.Now().Format(setting.WorkflowTimeFormat), errScanner.Text())
			}
		}()

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to login docker registry: %s %s", err, out.String())
		}

		log.Infof("Login ended. Duration: %.2f seconds.", time.Since(startTimeDockerLogin).Seconds())
	}
	return nil
}

func (s *DockerBuildStep) runDockerBuild() error {
	if s.spec == nil {
		return nil
	}

	log.Infof("Preparing Dockerfile.")
	startTimePrepareDockerfile := time.Now()
	err := prepareDockerfile(s.spec.Source, s.spec.DockerTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to prepare dockerfile: %s", err)
	}
	log.Infof("Preparation ended. Duration: %.2f seconds.", time.Since(startTimePrepareDockerfile).Seconds())

	if s.spec.Proxy != nil {
		setProxy(s.spec)
	}

	log.Infof("Running Docker Build.")
	startTimeDockerBuild := time.Now()
	envs := s.envs
	for _, c := range s.dockerCommands() {

		cmdOutReader, err := c.StdoutPipe()
		if err != nil {
			return err
		}

		outScanner := bufio.NewScanner(cmdOutReader)
		go func() {
			for outScanner.Scan() {
				fmt.Printf("%s   %s\n", time.Now().Format(setting.WorkflowTimeFormat), outScanner.Text())
			}
		}()

		cmdErrReader, err := c.StderrPipe()
		if err != nil {
			return err
		}

		errScanner := bufio.NewScanner(cmdErrReader)
		go func() {
			for errScanner.Scan() {
				fmt.Printf("%s   %s\n", time.Now().Format(setting.WorkflowTimeFormat), errScanner.Text())
			}
		}()

		c.Dir = s.workspace
		c.Env = envs
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to run docker build: %s", err)
		}
	}
	log.Infof("Docker build ended. Duration: %.2f seconds.", time.Since(startTimeDockerBuild).Seconds())

	return nil
}

func (s *DockerBuildStep) dockerCommands() []*exec.Cmd {
	cmds := make([]*exec.Cmd, 0)
	if s.spec.WorkDir == "" {
		s.spec.WorkDir = "."
	}

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

func dockerInitBuildxCmd(platform string, buildKitImage string) *exec.Cmd {
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

func dockerLogin(user, password, registry string) *exec.Cmd {
	return exec.Command(
		dockerExe,
		"login",
		"-u", user,
		"-p", password,
		registry,
	)
}

func prepareDockerfile(dockerfileSource, dockerfileContent string) error {
	if dockerfileSource == setting.DockerfileSourceTemplate {
		reader := strings.NewReader(dockerfileContent)
		readCloser := io.NopCloser(reader)
		path := fmt.Sprintf("/%s", setting.ZadigDockerfilePath)
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
