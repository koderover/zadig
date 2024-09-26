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

package archive

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/koderover/obelisk"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/helper"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type ArchiveHtmlStep struct {
	spec       *step.StepArchiveHtmlSpec
	envs       []string
	secretEnvs []string
	workspace  string
	logger     *log.JobLogger
	dirs       *types.AgentWorkDirs
}

func NewArchiveHtmlStep(spec interface{}, dirs *types.AgentWorkDirs, envs, secretEnvs []string, logger *log.JobLogger) (*ArchiveHtmlStep, error) {
	archiveHtmlStep := &ArchiveHtmlStep{dirs: dirs, workspace: dirs.Workspace, envs: envs, secretEnvs: secretEnvs, logger: logger}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return archiveHtmlStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &archiveHtmlStep.spec); err != nil {
		return archiveHtmlStep, fmt.Errorf("unmarshal spec %s to archive html spec failed", yamlBytes)
	}
	return archiveHtmlStep, nil
}

func (s *ArchiveHtmlStep) Run(ctx context.Context) error {
	start := time.Now()
	defer func() {
		s.logger.Infof(fmt.Sprintf("Archive html ended. Duration: %.2f seconds", time.Since(start).Seconds()))
	}()

	req := obelisk.Request{URL: fmt.Sprintf("file://%s", s.spec.HtmlPath)}
	arc := obelisk.Archiver{EnableLog: true}
	arc.Validate()

	envmaps := helper.MakeEnvMap(s.envs, s.secretEnvs)
	s.logger.Infof(fmt.Sprintf("Start archive html %s.", helper.ReplaceEnvWithValue(s.spec.HtmlPath, envmaps)))

	s.spec.HtmlPath = fmt.Sprintf("$env:WORKSPACE/%s", s.spec.HtmlPath)
	s.spec.HtmlPath = helper.ReplaceEnvWithValue(s.spec.HtmlPath, envmaps)
	s.spec.OutputPath = fmt.Sprintf("$env:WORKSPACE/%s", s.spec.OutputPath)
	s.spec.OutputPath = helper.ReplaceEnvWithValue(s.spec.OutputPath, envmaps)

	if runtime.GOOS == "windows" {
		s.spec.HtmlPath = filepath.FromSlash(filepath.ToSlash(s.spec.HtmlPath))
		s.spec.HtmlPath = strings.TrimSpace(s.spec.HtmlPath)
		s.spec.OutputPath = filepath.FromSlash(filepath.ToSlash(s.spec.OutputPath))
		s.spec.OutputPath = strings.TrimSpace(s.spec.OutputPath)
	}

	log.Infof("Start archive html %s.", s.spec.HtmlPath)
	result, _, err := arc.Archive(context.Background(), req)
	if err != nil {
		return fmt.Errorf("archive html failed, error: %v", err)
	}

	err = os.WriteFile(s.spec.OutputPath, result, 0644)
	if err != nil {
		return fmt.Errorf("write archive html to %s failed, error: %v", s.spec.OutputPath, err)
	}

	return nil
}
