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

type SharedCachePublishStep struct {
	spec *typesstep.StepSharedCachePublishSpec
}

func NewSharedCachePublishStep(spec interface{}) (*SharedCachePublishStep, error) {
	publishStep := &SharedCachePublishStep{}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return publishStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &publishStep.spec); err != nil {
		return publishStep, fmt.Errorf("unmarshal spec %s to shared cache publish spec failed", yamlBytes)
	}
	return publishStep, nil
}

func (s *SharedCachePublishStep) Run(ctx context.Context) error {
	log.Infof("Start publishing shared cache to %s.", s.spec.TaskDir)
	if _, err := os.Stat(s.spec.CacheDir); err != nil {
		if os.IsNotExist(err) {
			log.Infof("Shared cache publish skipped because cache dir %s does not exist.", s.spec.CacheDir)
			return nil
		}
		return s.handleErr(fmt.Errorf("stat cache dir failed: %w", err))
	}
	_ = os.RemoveAll(s.spec.TaskDir)
	if err := copyDirContent(s.spec.CacheDir, s.spec.TaskDir); err != nil {
		return s.handleErr(fmt.Errorf("copy cache content to task dir failed: %w", err))
	}
	if err := copyDirContent(s.spec.TaskDir, s.spec.MergedDir); err != nil {
		return s.handleErr(fmt.Errorf("merge task cache into shared cache dir failed: %w", err))
	}
	log.Infof("Shared cache publish finished.")
	return nil
}

func (s *SharedCachePublishStep) handleErr(err error) error {
	if err == nil {
		return nil
	}
	if s.spec.IgnoreErr {
		log.Errorf("shared cache publish failed: %v", err)
		return nil
	}
	return err
}
