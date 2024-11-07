/*
Copyright 2024 The KodeRover Authors.

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
	"context"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	c "github.com/koderover/zadig/v2/pkg/microservice/jobexecutor/core/service/cmd"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type P4Step struct {
	spec       *step.StepP4Spec
	envs       []string
	secretEnvs []string
	workspace  string
}

func NewP4Step(spec interface{}, workspace string, envs, secretEnvs []string) (*P4Step, error) {
	p4Step := &P4Step{workspace: workspace, envs: envs, secretEnvs: secretEnvs}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return p4Step, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &p4Step.spec); err != nil {
		return p4Step, fmt.Errorf("unmarshal spec %s to perforce spec failed", yamlBytes)
	}
	return p4Step, nil
}

func (s *P4Step) Run(ctx context.Context) error {
	start := time.Now()
	log.Infof("Start perforce sync.")
	defer func() {
		log.Infof("perforce sync ended. Duration: %.2f seconds.", time.Since(start).Seconds())
	}()
	err := s.syncPerforceWorkspace()
	if err != nil {
		log.Infof("sync perforce code error: %s", err)
	}

	return nil
}

func (s *P4Step) syncPerforceWorkspace() error {
	for i, repo := range s.spec.Repos {
		log.Infof("sync perforce code host: %s", repo.PerforceHost)
		log.Infof("sync perforce code port: %d", repo.PerforcePort)
		log.Infof("sync perforce code username: %s", repo.Username)
		log.Infof("sync perforce code password: %s", repo.Password)
		err := s.syncP4Depot(s.envs, repo, i)
		if err != nil {
			repoName := ""
			if repo.Stream != "" {
				repoName = repo.Stream
			} else {
				repoName = repo.ViewMapping
			}
			log.Errorf("sync p4 depot: %s error: %s", repoName, err)
			return fmt.Errorf("sync p4 depot: %s error: %s", repoName, err)
		}
	}

	return nil
}

func (s *P4Step) syncP4Depot(envs []string, repo *types.Repository, repoIndex int) error {
	finalCmds := make([]*c.Command, 0)
	if repo == nil {
		return fmt.Errorf("nil repository given to p4 depot job")
	}
	if repo.Source != types.ProviderPerforce {
		return fmt.Errorf("repo type is not perforce")
	}

	envCopy := make([]string, 0)

	for _, env := range envs {
		envCopy = append(envCopy, env)
	}

	loginCmds := c.PerforceLogin(repo.PerforceHost, repo.PerforcePort, repo.Username, repo.Password)

	for _, cmd := range loginCmds {
		finalCmds = append(finalCmds, &c.Command{
			Cmd:          cmd,
			DisableTrace: true,
		})
	}

	clientName := fmt.Sprintf("%s_%s_%d_ch%d_%s", s.spec.JobName, s.spec.WorkflowName, s.spec.TaskID, repoIndex, s.spec.ProjectKey)

	createWorkspaceCmds := c.PerforceCreateWorkspace(clientName, repo.DepotType, repo.Stream, repo.ViewMapping)
	for _, cmd := range createWorkspaceCmds {
		finalCmds = append(finalCmds, &c.Command{
			Cmd:          cmd,
			DisableTrace: true,
		})
	}

	syncCodeCmd := c.PerforceSync(clientName, repo.ChangeListID)
	for _, cmd := range syncCodeCmd {
		finalCmds = append(finalCmds, &c.Command{
			Cmd:          cmd,
			DisableTrace: true,
		})
	}

	unshelveCmd := c.PerforceUnshelve(clientName, repo.ShelveID)
	for _, cmd := range unshelveCmd {
		finalCmds = append(finalCmds, &c.Command{
			Cmd:          cmd,
			DisableTrace: true,
		})
	}

	var tokens []string

	if repo == nil {
		tokens = append(tokens, repo.Password)
	}

	for _, command := range finalCmds {
		cmdOutReader, err := command.Cmd.StdoutPipe()
		if err != nil {
			return err
		}

		outScanner := bufio.NewScanner(cmdOutReader)
		go func() {
			for outScanner.Scan() {
				fmt.Printf("%s   %s\n", time.Now().Format(setting.WorkflowTimeFormat), maskSecret(tokens, outScanner.Text()))
			}
		}()

		cmdErrReader, err := command.Cmd.StderrPipe()
		if err != nil {
			return err
		}

		errScanner := bufio.NewScanner(cmdErrReader)
		go func() {
			for errScanner.Scan() {
				fmt.Printf("%s   %s\n", time.Now().Format(setting.WorkflowTimeFormat), maskSecret(tokens, errScanner.Text()))
			}
		}()

		command.Cmd.Env = envs
		if !command.DisableTrace {
			fmt.Printf("%s   %s\n", time.Now().Format(setting.WorkflowTimeFormat), strings.Join(command.Cmd.Args, " "))
		}
		if err := command.Cmd.Run(); err != nil {
			if command.IgnoreError {
				continue
			}
			return err
		}
	}

	return nil
}
