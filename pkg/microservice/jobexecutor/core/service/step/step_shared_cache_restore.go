/*
Copyright 2026 The KodeRover Authors.

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
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/tool/log"
	typesstep "github.com/koderover/zadig/v2/pkg/types/step"
)

type SharedCacheRestoreStep struct {
	spec *typesstep.StepSharedCacheRestoreSpec
}

func NewSharedCacheRestoreStep(spec interface{}) (*SharedCacheRestoreStep, error) {
	restoreStep := &SharedCacheRestoreStep{}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return restoreStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &restoreStep.spec); err != nil {
		return restoreStep, fmt.Errorf("unmarshal spec %s to shared cache restore spec failed", yamlBytes)
	}
	return restoreStep, nil
}

func (s *SharedCacheRestoreStep) Run(ctx context.Context) error {
	log.Infof("Start restoring shared cache from %s.", s.spec.MergedDir)
	if _, err := os.Stat(s.spec.MergedDir); err != nil {
		if os.IsNotExist(err) {
			log.Infof("Shared cache restore skipped because merged cache dir %s does not exist.", s.spec.MergedDir)
			return nil
		}
		return fmt.Errorf("stat merged cache dir failed: %w", err)
	}
	if err := copyDirContent(s.spec.MergedDir, s.spec.CacheDir); err != nil {
		return fmt.Errorf("restore shared cache failed: %w", err)
	}
	log.Infof("Shared cache restore finished.")
	return nil
}
