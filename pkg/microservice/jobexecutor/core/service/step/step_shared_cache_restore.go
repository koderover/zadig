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
	"path/filepath"

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
	if s.spec.SkipContent {
		log.Infof("Start recording shared cache base version from %s without restoring cache content.", s.spec.StoreDir)
	} else {
		log.Infof("Start restoring shared cache from %s.", s.spec.StoreDir)
	}
	if err := os.MkdirAll(s.spec.StoreDir, os.ModePerm); err != nil {
		return s.handleErr(fmt.Errorf("create store dir failed: %w", err))
	}
	currentFile := filepath.Join(s.spec.StoreDir, "current.json")
	current, found, err := loadSharedCacheCurrent(currentFile)
	if err != nil {
		return s.handleErr(fmt.Errorf("load current cache metadata failed: %w", err))
	}
	if !found {
		log.Infof("Shared cache restore skipped because current cache metadata does not exist.")
		return s.handleErr(writeSharedCacheRestoreMetadata(s.spec.MetadataFile, "", false))
	}

	markerFile := ""
	if !s.spec.SkipContent {
		var err error
		markerFile, err = createSharedCacheActiveMarker(s.spec.StoreDir, current.Version, "restore")
		if err != nil {
			return s.handleErr(fmt.Errorf("create shared cache restore marker failed: %w", err))
		}
		defer removeSharedCacheActiveMarker(markerFile)
	}

	snapshotDir := filepath.Join(s.spec.StoreDir, current.SnapshotDir)
	if _, err := os.Stat(snapshotDir); err != nil {
		if os.IsNotExist(err) {
			log.Infof("Shared cache restore skipped because snapshot dir %s does not exist.", snapshotDir)
			return s.handleErr(writeSharedCacheRestoreMetadata(s.spec.MetadataFile, "", false))
		}
		return s.handleErr(fmt.Errorf("stat snapshot dir failed: %w", err))
	}
	if err := writeSharedCacheRestoreMetadata(s.spec.MetadataFile, current.Version, true); err != nil {
		return s.handleErr(fmt.Errorf("write restore metadata failed: %w", err))
	}
	if s.spec.SkipContent {
		log.Infof("Shared cache content restore skipped with base version %s.", current.Version)
		return nil
	}

	if err := copyDirContent(ctx, snapshotDir, s.spec.CacheDir); err != nil {
		return s.handleErr(err)
	}
	log.Infof("Shared cache restore finished with version %s.", current.Version)
	return nil
}

func (s *SharedCacheRestoreStep) handleErr(err error) error {
	if err == nil {
		return nil
	}
	if s.spec.IgnoreErr {
		log.Errorf("shared cache restore failed, storeDir: %s, cacheDir: %s, metadataFile: %s, skipContent: %v, err: %v",
			s.spec.StoreDir, s.spec.CacheDir, s.spec.MetadataFile, s.spec.SkipContent, err)
		return nil
	}
	return err
}
