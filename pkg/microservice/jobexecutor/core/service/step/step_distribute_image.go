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
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types/step"
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
		return distributeImageStep, fmt.Errorf("unmarshal spec %s to distribute image spec failed", yamlBytes)
	}
	return distributeImageStep, nil
}

func (s *DistributeImageStep) Run(ctx context.Context) error {
	log.Info("Start distribute images.")
	if s.spec.SourceRegistry == nil || s.spec.TargetRegistry == nil {
		return errors.New("image registry infos are missing")
	}

	wg := sync.WaitGroup{}
	errList := new(multierror.Error)
	errLock := sync.Mutex{}
	appendError := func(err error) {
		errLock.Lock()
		defer errLock.Unlock()
		errList = multierror.Append(errList, err)
	}

	if s.spec.Type == config.DistributeImageMethodCloudSync {
		log.Info("Distribute images type is cloud sync, try to pull image.")

		if err := s.loginTargetRegistry(); err != nil {
			return err
		}
		for _, target := range s.spec.DistributeTarget {
			wg.Add(1)
			go func(target *step.DistributeTaskTarget) {
				defer wg.Done()
				pushCmd := dockerPullCmd(target.TargetImage, s.spec.Architecture)
				out := bytes.Buffer{}
				pushCmd.Stdout = &out
				pushCmd.Stderr = &out
				if err := pushCmd.Run(); err != nil {
					errMsg := fmt.Sprintf("failed to pull image: %s %s", err, out.String())
					appendError(errors.New(errMsg))
					return
				}
				log.Infof("pull image [%s] succeed", target.TargetImage)
			}(target)
		}
		wg.Wait()
		if err := errList.ErrorOrNil(); err != nil {
			return fmt.Errorf("pull target images error: %v", err)
		}
		return nil
	} else {
		if err := s.loginSourceRegistry(); err != nil {
			return err
		}

		for _, target := range s.spec.DistributeTarget {
			wg.Add(1)
			go func(target *step.DistributeTaskTarget) {
				defer wg.Done()
				pullCmd := dockerPullCmd(target.SourceImage, s.spec.Architecture)
				out := bytes.Buffer{}
				pullCmd.Stdout = &out
				pullCmd.Stderr = &out
				if err := pullCmd.Run(); err != nil {
					errMsg := fmt.Sprintf("failed to pull image: %s %s", err, out.String())
					appendError(errors.New(errMsg))
					return
				}
				log.Infof("pull source image [%s] succeed", target.SourceImage)

				tagCmd := dockerTagCmd(target.SourceImage, target.TargetImage)
				out = bytes.Buffer{}
				tagCmd.Stdout = &out
				tagCmd.Stderr = &out
				if err := tagCmd.Run(); err != nil {
					errMsg := fmt.Sprintf("failed to tag image: %s %s", err, out.String())
					appendError(errors.New(errMsg))
					return
				}
				log.Infof("tag image [%s] to [%s] succeed", target.SourceImage, target.TargetImage)
			}(target)
		}
		wg.Wait()
		if err := errList.ErrorOrNil(); err != nil {
			return fmt.Errorf("prepare source images error: %v", err)
		}
		log.Infof("Finish prepare source images.")

		if err := s.loginTargetRegistry(); err != nil {
			return err
		}
		for _, target := range s.spec.DistributeTarget {
			wg.Add(1)
			go func(target *step.DistributeTaskTarget) {
				defer wg.Done()
				pushCmd := dockerPush(target.TargetImage)
				out := bytes.Buffer{}
				pushCmd.Stdout = &out
				pushCmd.Stderr = &out
				if err := pushCmd.Run(); err != nil {
					errMsg := fmt.Sprintf("failed to push image: %s %s", err, out.String())
					appendError(errors.New(errMsg))
					return
				}
				log.Infof("push image [%s] succeed", target.TargetImage)
			}(target)
		}
		wg.Wait()
		if err := errList.ErrorOrNil(); err != nil {
			return fmt.Errorf("push target images error: %v", err)
		}

	}

	log.Info("Finish distribute images.")
	return nil
}

func (s *DistributeImageStep) loginSourceRegistry() error {
	log.Info("Logging in Docker Source Registry.")
	startTimeDockerLogin := time.Now()
	loginCmd := dockerLogin(s.spec.SourceRegistry.AccessKey, s.spec.SourceRegistry.SecretKey, s.spec.SourceRegistry.RegAddr)
	var out bytes.Buffer
	loginCmd.Stdout = &out
	loginCmd.Stderr = &out
	if err := loginCmd.Run(); err != nil {
		return fmt.Errorf("failed to login docker registry: %s %s", err, out.String())
	}
	log.Infof("Login ended. Duration: %.2f seconds.", time.Since(startTimeDockerLogin).Seconds())
	return nil
}

func (s *DistributeImageStep) loginTargetRegistry() error {
	log.Info("Logging in Docker Target Registry.")
	startTimeDockerLogin := time.Now()
	loginCmd := dockerLogin(s.spec.TargetRegistry.AccessKey, s.spec.TargetRegistry.SecretKey, s.spec.TargetRegistry.RegAddr)
	var out bytes.Buffer
	loginCmd.Stdout = &out
	loginCmd.Stderr = &out
	if err := loginCmd.Run(); err != nil {
		return fmt.Errorf("failed to login docker registry: %s %s", err, out.String())
	}
	log.Infof("Login ended. Duration: %.2f seconds.", time.Since(startTimeDockerLogin).Seconds())
	return nil
}

func dockerPullCmd(fullImage string, architecture string) *exec.Cmd {
	args := []string{"-c"}
	dockerPullCommand := fmt.Sprintf("docker pull %s", fullImage)
	if architecture != "" {
		dockerPullCommand += fmt.Sprintf(" --platform=%s", architecture)
	}
	args = append(args, dockerPullCommand)
	return exec.Command("sh", args...)
}

func dockerTagCmd(sourceImage, targetImage string) *exec.Cmd {
	args := []string{"-c"}
	dockerTagCommand := fmt.Sprintf("docker tag %s %s", sourceImage, targetImage)
	args = append(args, dockerTagCommand)
	return exec.Command("sh", args...)
}
