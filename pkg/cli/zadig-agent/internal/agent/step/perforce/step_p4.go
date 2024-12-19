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

package perforce

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/perforce"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/helper"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	agenttypes "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

type P4Step struct {
	spec       *step.StepGitSpec
	envs       []string
	secretEnvs []string
	dirs       *agenttypes.AgentWorkDirs
	Logger     *log.JobLogger
}

func NewP4Step(spec interface{}, dirs *agenttypes.AgentWorkDirs, envs, secretEnvs []string, logger *log.JobLogger) (*P4Step, error) {
	p4Step := &P4Step{dirs: dirs, envs: envs, secretEnvs: secretEnvs, Logger: logger}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return p4Step, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &p4Step.spec); err != nil {
		return p4Step, fmt.Errorf("unmarshal spec %s to git spec failed", yamlBytes)
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
		return fmt.Errorf("sync perforce code error: %s", err)
	}

	return nil
}

func (s *P4Step) syncPerforceWorkspace() error {
	for _, repo := range s.spec.Repos {
		err := syncP4Depot(s.envs, s.secretEnvs, repo, s.Logger)
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

func syncP4Depot(envs, secretEnvs []string, repo *types.Repository, logger *log.JobLogger) error {
	finalCmds := make([]*common.Command, 0)
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

	loginCmds := perforce.PerforceLogin(repo.PerforceHost, repo.PerforcePort, repo.Username, repo.Password)

	for _, cmd := range loginCmds {
		finalCmds = append(finalCmds, &common.Command{
			Cmd:          cmd,
			DisableTrace: true,
		})
	}

	// TODO: generate client name by depot/codehost info to make it unique
	clientName := "zadig-client"

	createWorkspaceCmds := perforce.PerforceCreateWorkspace(clientName, repo.DepotType, repo.Stream, repo.ViewMapping)
	for _, cmd := range createWorkspaceCmds {
		finalCmds = append(finalCmds, &common.Command{
			Cmd:          cmd,
			DisableTrace: true,
		})
	}

	syncCodeCmd := perforce.PerforceSync(clientName, repo.ChangeListID)
	for _, cmd := range syncCodeCmd {
		finalCmds = append(finalCmds, &common.Command{
			Cmd:          cmd,
			DisableTrace: true,
		})
	}

	unshelveCmd := perforce.PerforceUnshelve(clientName, repo.ShelveID)
	for _, cmd := range unshelveCmd {
		finalCmds = append(finalCmds, &common.Command{
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
		cmdErrReader, err := command.Cmd.StderrPipe()
		if err != nil {
			return err
		}

		command.Cmd.Env = envs
		if !command.DisableTrace {
			logger.Printf("%s\n", util.MaskSecretEnvs(strings.Join(command.Cmd.Args, " "), secretEnvs))
		}
		if err := command.Cmd.Start(); err != nil {
			if command.IgnoreError {
				continue
			}
			return err
		}

		var wg sync.WaitGroup
		needPersistentLog := true
		// write script output to log file
		wg.Add(1)
		go func() {
			defer wg.Done()

			helper.HandleCmdOutput(cmdOutReader, needPersistentLog, logger.GetLogfilePath(), secretEnvs, log.GetSimpleLogger())
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()

			helper.HandleCmdOutput(cmdErrReader, needPersistentLog, logger.GetLogfilePath(), secretEnvs, log.GetSimpleLogger())
		}()

		wg.Wait()
		if err := command.Cmd.Wait(); err != nil {
			if command.IgnoreError {
				continue
			}
			return err
		}
	}

	return nil
}
