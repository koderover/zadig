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
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/pkg/microservice/packager/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

// Packager ...
type Packager struct {
	Ctx *Context
}

type PackageResult struct {
	ServiceName string       `json:"service_name"`
	Result      string       `json:"result"`
	ErrorMsg    string       `json:"error_msg"`
	ImageData   []*ImageData `json:"image_data"`
}

func NewPackager() (*Packager, error) {
	context, err := ioutil.ReadFile(config.JobConfigFile())
	if err != nil {
		return nil, err
	}

	var ctx *Context
	if err := yaml.Unmarshal(context, &ctx); err != nil {
		return nil, err
	}

	packager := &Packager{
		Ctx: ctx,
	}

	return packager, nil
}

// BeforeExec ...
func (p *Packager) BeforeExec() error {
	log.Info("wait for docker daemon to start")
	for i := 0; i < 120; i++ {
		err := dockerInfo().Run()
		if err == nil {
			break
		}
		time.Sleep(time.Second * 1)
	}

	if len(p.Ctx.ProgressFile) == 0 {
		return nil
	}

	// create log file
	if err := os.MkdirAll(filepath.Dir(p.Ctx.ProgressFile), 0770); err != nil {
		return errors.Wrapf(err, "failed to create progress file dir")
	}
	file, err := os.Create(p.Ctx.ProgressFile)
	if err != nil {
		return errors.Wrapf(err, "failed to create progress file")
	}
	err = file.Close()
	return errors.Wrapf(err, "failed to close progress file")
}

// Exec ...
func (p *Packager) Exec() error {

	// add docker registry auths for all registries
	allRegistries := make([]*DockerRegistry, 0)
	allRegistries = append(allRegistries, p.Ctx.SourceRegistries...)
	allRegistries = append(allRegistries, p.Ctx.TargetRegistries...)
	log.Infof("all registry count %v", len(allRegistries))
	err := writeDockerConfig(allRegistries)
	if err != nil {
		return err
	}

	realTimeProgress := make([]*PackageResult, 0)

	for _, image := range p.Ctx.Images {
		cmds := p.dockerCommands(image)
		var err error
		for _, cmd := range cmds {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			log.Info(strings.Join(cmd.Args, " "))
			if err = cmd.Run(); err != nil {
				break
			}
		}

		result := &PackageResult{
			ServiceName: image.ServiceName,
		}

		if err != nil {
			result.Result = "failed"
			result.ErrorMsg = err.Error()
			log.Infof("[result][fail][%s][%s]", image.ServiceName, err)
		} else {
			result.Result = "success"
			result.ImageData = image.Images
			log.Infof("[result][success][%s]", image.ServiceName)
		}

		realTimeProgress = append(realTimeProgress, result)

		if len(p.Ctx.ProgressFile) > 0 {
			bs, err := json.Marshal(realTimeProgress)
			if err != nil {
				log.Errorf("failed to marshal progress data %s", err)
				continue
			}
			err = os.WriteFile(p.Ctx.ProgressFile, bs, 0644)
			if err != nil {
				log.Errorf("failed to write progress data %s", err)
				continue
			}
		}
	}

	time.Sleep(time.Second * 10)
	return nil
}

func writeDockerConfig(registries []*DockerRegistry) error {

	authMap := make(map[string]map[string]string)
	//host string, username string, password string
	for _, registry := range registries {
		if registry.UserName == "" {
			continue
		}
		authMap[registry.Host] = map[string]string{
			"auth": base64.StdEncoding.EncodeToString(
				[]byte(strings.Join([]string{registry.UserName, registry.Password}, ":")),
			)}
	}

	dir := path.Join(config.Home(), ".docker")
	if err := os.MkdirAll(dir, 0600); err != nil {
		return err
	}

	cfg := map[string]map[string]map[string]string{
		"auths": authMap,
	}

	data, err := json.Marshal(cfg)

	if err != nil {
		return err
	}

	return ioutil.WriteFile(path.Join(dir, "config.json"), data, 0600)
}

func (p *Packager) dockerCommands(imageByService *ImagesByService) []*exec.Cmd {
	cmds := make([]*exec.Cmd, 0)
	//cmds = append(cmds, dockerVersion())

	buildTargetImage := func(imageName, imageTag, host, nameSpace string) string {
		ret := ""
		if len(nameSpace) > 0 {
			ret = fmt.Sprintf("%s/%s/%s:%s", host, nameSpace, imageName, imageTag)
		} else {
			ret = fmt.Sprintf("%s/%s:%s", host, imageName, imageTag)
		}
		ret = strings.TrimPrefix(ret, "http://")
		ret = strings.TrimPrefix(ret, "https://")
		return ret
	}

	if p.Ctx.JobType == setting.BuildChartPackage {
		for _, image := range imageByService.Images {
			for _, registry := range p.Ctx.TargetRegistries {
				cmds = append(cmds, dockerPull(image.ImageUrl))
				targetImage := buildTargetImage(image.ImageName, image.ImageTag, registry.Host, registry.Namespace)
				cmds = append(cmds, dockerTag(image.ImageUrl, targetImage))
				cmds = append(cmds, dockerPush(targetImage))
				image.ImageUrl = targetImage
			}
		}
	}

	return cmds
}
