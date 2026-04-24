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
	"time"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/kube/lease"
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
	log.Infof("Start publishing shared cache to %s.", s.spec.StoreDir)
	if _, err := os.Stat(s.spec.CacheDir); err != nil {
		if os.IsNotExist(err) {
			log.Infof("Shared cache publish skipped because cache dir %s does not exist.", s.spec.CacheDir)
			return nil
		}
		return s.handleErr(fmt.Errorf("stat cache dir failed: %w", err))
	}
	if err := os.MkdirAll(filepath.Join(s.spec.StoreDir, "snapshots"), os.ModePerm); err != nil {
		return s.handleErr(fmt.Errorf("create snapshot dir failed: %w", err))
	}

	snapshotsDir := filepath.Join(s.spec.StoreDir, "snapshots")
	tempSnapshotDir := filepath.Join(snapshotsDir, ".tmp-"+s.spec.Version)
	finalSnapshotDir := filepath.Join(snapshotsDir, s.spec.Version)
	_ = os.RemoveAll(tempSnapshotDir)
	if err := os.MkdirAll(tempSnapshotDir, os.ModePerm); err != nil {
		return s.handleErr(fmt.Errorf("create temp snapshot dir failed: %w", err))
	}
	if err := copyDirContent(s.spec.CacheDir, tempSnapshotDir); err != nil {
		_ = os.RemoveAll(tempSnapshotDir)
		return s.handleErr(err)
	}
	if err := os.Rename(tempSnapshotDir, finalSnapshotDir); err != nil {
		_ = os.RemoveAll(tempSnapshotDir)
		return s.handleErr(fmt.Errorf("promote temp snapshot dir failed: %w", err))
	}

	leaseDuration := time.Duration(s.spec.LeaseDurationSeconds) * time.Second
	lock, err := lease.NewLock(s.spec.LeaseName, leaseDuration)
	if err != nil {
		return s.handleErr(fmt.Errorf("create shared cache publish lease lock failed: %w", err))
	}
	if err := lock.Acquire(ctx); err != nil {
		return s.handleErr(fmt.Errorf("acquire shared cache publish lease lock failed: %w", err))
	}
	defer func() {
		if err := lock.Release(context.Background()); err != nil {
			log.Errorf("release shared cache publish lease lock failed: %v", err)
		}
	}()

	restoreMeta, _, err := loadSharedCacheRestoreMetadata(s.spec.MetadataFile)
	if err != nil {
		return s.handleErr(fmt.Errorf("load restore metadata failed: %w", err))
	}

	currentFile := filepath.Join(s.spec.StoreDir, "current.json")
	current, found, err := loadSharedCacheCurrent(currentFile)
	if err != nil {
		return s.handleErr(fmt.Errorf("load current cache metadata failed: %w", err))
	}
	if found && restoreMeta != nil && restoreMeta.BaseVersion != "" && current.Version != restoreMeta.BaseVersion {
		log.Infof("Shared cache publish skipped current pointer update because base version %s is stale, latest version is %s.", restoreMeta.BaseVersion, current.Version)
		if err := cleanupSharedCacheSnapshots(snapshotsDir, current.Version, setting.SharedCacheSnapshotRetain); err != nil {
			return s.handleErr(fmt.Errorf("cleanup old shared cache snapshots failed: %w", err))
		}
		return nil
	}

	current = &sharedCacheCurrent{
		Version:         s.spec.Version,
		SnapshotDir:     filepath.Join("snapshots", s.spec.Version),
		UpdatedAt:       time.Now().Unix(),
		UpdatedByTaskID: s.spec.TaskID,
		WorkflowName:    s.spec.WorkflowName,
		JobName:         s.spec.JobName,
	}
	if err := writeSharedCacheCurrent(currentFile, current); err != nil {
		return s.handleErr(fmt.Errorf("write current cache metadata failed: %w", err))
	}
	if err := cleanupSharedCacheSnapshots(snapshotsDir, current.Version, setting.SharedCacheSnapshotRetain); err != nil {
		return s.handleErr(fmt.Errorf("cleanup old shared cache snapshots failed: %w", err))
	}
	log.Infof("Shared cache publish finished with version %s.", s.spec.Version)
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
