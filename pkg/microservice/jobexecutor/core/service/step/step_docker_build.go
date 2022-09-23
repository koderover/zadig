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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types/step"
	"github.com/koderover/zadig/pkg/util/fs"
	"gopkg.in/yaml.v3"
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
	start := time.Now()
	log.Infof("Start docker build.")
	defer func() {
		log.Infof("Docker build ended. Duration: %.2f seconds.", time.Since(start).Seconds())
	}()

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
		fmt.Printf("Logining Docker Registry: %s.\n", s.spec.DockerRegistry.Host)
		startTimeDockerLogin := time.Now()
		cmd := dockerLogin(s.spec.DockerRegistry.UserName, s.spec.DockerRegistry.Password, s.spec.DockerRegistry.Host)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to login docker registry: %s %s", err, out.String())
		}

		fmt.Printf("Login ended. Duration: %.2f seconds.\n", time.Since(startTimeDockerLogin).Seconds())
	}
	return nil
}

func (s *DockerBuildStep) runDockerBuild() error {
	if s.spec == nil {
		return nil
	}

	fmt.Printf("Preparing Dockerfile.\n")
	startTimePrepareDockerfile := time.Now()
	err := prepareDockerfile(s.spec.Source, s.spec.DockerTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to prepare dockerfile: %s", err)
	}
	fmt.Printf("Preparation ended. Duration: %.2f seconds.\n", time.Since(startTimePrepareDockerfile).Seconds())

	if s.spec.Proxy != nil {
		setProxy(s.spec)
	}

	fmt.Printf("Runing Docker Build.\n")
	startTimeDockerBuild := time.Now()
	envs := s.envs
	for _, c := range s.dockerCommands() {
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Dir = s.workspace
		c.Env = envs
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to run docker build: %s", err)
		}
	}
	fmt.Printf("Docker build ended. Duration: %.2f seconds.\n", time.Since(startTimeDockerBuild).Seconds())

	return nil
}

func (s *DockerBuildStep) dockerCommands() []*exec.Cmd {
	cmds := make([]*exec.Cmd, 0)
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
	args := []string{
		"push",
		fullImage,
	}
	return exec.Command(dockerExe, args...)
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
