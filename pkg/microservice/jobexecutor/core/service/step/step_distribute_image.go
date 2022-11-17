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
	"errors"
	"fmt"
	"sync"

	"gopkg.in/yaml.v2"

	"github.com/hashicorp/go-multierror"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types/step"
	"github.com/regclient/regclient"
	"github.com/regclient/regclient/config"
	"github.com/regclient/regclient/types/ref"
)

type DistributeImageStep struct {
	spec       *step.StepImageDistributeSpec
	envs       []string
	secretEnvs []string
	workspace  string
}

func NewDistributeImageStep(spec interface{}, workspace string, envs, secretEnvs []string) (*DistributeImageStep, error) {
	distributeImageStep := &DistributeImageStep{workspace: workspace, envs: envs, secretEnvs: secretEnvs}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return distributeImageStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &distributeImageStep.spec); err != nil {
		return distributeImageStep, fmt.Errorf("unmarshal spec %s to shell spec failed", yamlBytes)
	}
	return distributeImageStep, nil
}

func (s *DistributeImageStep) Run(ctx context.Context) error {
	log.Info("Start distribute images.")
	if s.spec.SourceRegistry == nil || s.spec.TargetRegistry == nil {
		return errors.New("image registry infos are missing")
	}
	sourceHost := config.HostNewName(s.spec.SourceRegistry.RegAddr)
	sourceHost.User = s.spec.SourceRegistry.AccessKey
	sourceHost.Pass = s.spec.SourceRegistry.SecretKey

	targetHost := config.HostNewName(s.spec.TargetRegistry.RegAddr)
	targetHost.User = s.spec.TargetRegistry.AccessKey
	targetHost.Pass = s.spec.TargetRegistry.SecretKey
	hostsOpt := regclient.WithConfigHosts([]config.Host{*sourceHost, *targetHost})
	client := regclient.New(hostsOpt)

	errList := new(multierror.Error)
	wg := sync.WaitGroup{}
	for _, target := range s.spec.DistributeTarget {
		wg.Add(1)
		go func(target *step.DistributeTaskTarget) {
			defer wg.Done()
			if err := copyImage(target, client); err != nil {
				errList = multierror.Append(errList, err)
			}
		}(target)
	}
	wg.Wait()
	if err := errList.ErrorOrNil(); err != nil {
		return fmt.Errorf("copy images error: %v", err)
	}
	log.Info("Finish distribute images.")
	return nil
}

func copyImage(target *step.DistributeTaskTarget, client *regclient.RegClient) error {
	sourceRef, err := ref.New(target.SoureImage)
	if err != nil {
		errMsg := fmt.Sprintf("parse source image: %s error: %v", target.SoureImage, err)
		return errors.New(errMsg)
	}
	targetRef, err := ref.New(target.TargetImage)
	if err != nil {
		errMsg := fmt.Sprintf("parse target image: %s error: %v", target.TargetImage, err)
		return errors.New(errMsg)
	}
	if err := client.ImageCopy(context.Background(), sourceRef, targetRef); err != nil {
		errMsg := fmt.Sprintf("copy image failed: %v", err)
		return errors.New(errMsg)
	}
	log.Infof("copy image from [%s] to [%s] succeed", target.SoureImage, target.SoureImage)
	return nil
}
