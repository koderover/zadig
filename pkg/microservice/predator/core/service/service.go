/*
Copyright 2021 The KodeRover Authors.

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

package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/pkg/microservice/predator/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

// Predator ...
type Predator struct {
	Ctx *Context
}

func NewPredator() (*Predator, error) {
	context, err := ioutil.ReadFile(config.JobConfigFile())
	if err != nil {
		return nil, err
	}

	var ctx *Context
	if err := yaml.Unmarshal(context, &ctx); err != nil {
		return nil, err
	}

	predator := &Predator{
		Ctx: ctx,
	}

	return predator, nil
}

// BeforeExec ...
func (p *Predator) BeforeExec() error {
	log.Info("wait for docker daemon to start")
	for i := 0; i < 120; i++ {
		err := dockerInfo().Run()
		if err == nil {
			break
		}
		time.Sleep(time.Second * 1)
	}

	if p.Ctx.OnSetup != "" {
		setUpCmd := exec.Command("bash", "-c", p.Ctx.OnSetup)
		setUpCmd.Stderr = os.Stderr
		setUpCmd.Stdout = os.Stdout
		log.Info(strings.Join(setUpCmd.Args, " "))
		if err := setUpCmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

// Exec ...
func (p *Predator) Exec() error {
	err := writeDockerConfig(p.Ctx.DockerRegistry.Host, p.Ctx.DockerRegistry.UserName, p.Ctx.DockerRegistry.Password)
	if err != nil {
		return err
	}

	cmds := p.dockerCommands()
	for _, cmd := range cmds {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = p.Ctx.DockerBuildCtx.WorkDir
		log.Info(strings.Join(cmd.Args, " "))
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

// AfterExec ...
func (p *Predator) AfterExec() error {
	for _, image := range p.Ctx.ReleaseImages {
		err := writeDockerConfig(image.Host, image.Username, image.Password)
		if err != nil {
			return fmt.Errorf("failed to write docker config file %v", err)
		}

		cmd := dockerPush(image.Name)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Dir = p.Ctx.DockerBuildCtx.WorkDir
		log.Info(strings.Join(cmd.Args, " "))
		if err := cmd.Run(); err != nil {
			return cmd.Run()
		}
	}

	return nil
}

func writeDockerConfig(host string, username string, password string) error {
	if username == "" {
		return nil
	}

	dir := path.Join(config.Home(), ".docker")
	if err := os.MkdirAll(dir, 0600); err != nil {
		return err
	}

	cfg := map[string]map[string]map[string]string{
		"auths": {
			host: {
				"auth": base64.StdEncoding.EncodeToString(
					[]byte(strings.Join([]string{username, password}, ":")),
				),
			},
		},
	}

	data, err := json.Marshal(cfg)

	if err != nil {
		return err
	}

	return ioutil.WriteFile(path.Join(dir, "config.json"), data, 0600)
}

func (p *Predator) dockerCommands() []*exec.Cmd {
	cmds := make([]*exec.Cmd, 0)
	cmds = append(cmds, dockerVersion())

	if p.Ctx.JobType == setting.BuildImageJob {
		cmds = append(cmds, dockerBuild(p.Ctx.DockerBuildCtx.GetDockerFile(), p.Ctx.DockerBuildCtx.ImageName, p.Ctx.DockerBuildCtx.BuildArgs))
		p.Ctx.ReleaseImages = append(p.Ctx.ReleaseImages, RepoImage{
			Name: p.Ctx.DockerBuildCtx.ImageName,
		})
	} else if p.Ctx.JobType == setting.ReleaseImageJob {
		cmds = append(cmds, dockerPull(p.Ctx.DockerBuildCtx.ImageName))
		for _, image := range p.Ctx.ReleaseImages {
			cmds = append(cmds, dockerTag(p.Ctx.DockerBuildCtx.ImageName, image.Name))
		}
	}

	return cmds
}
