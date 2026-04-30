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
	snapshotPromoted := false
	currentUpdated := false
	defer func() {
		if !snapshotPromoted || currentUpdated {
			return
		}
		if err := os.RemoveAll(finalSnapshotDir); err != nil {
			log.Errorf("remove unpublished shared cache snapshot %s failed: %v", finalSnapshotDir, err)
		}
	}()

	markerFile, err := createSharedCacheActiveMarker(s.spec.StoreDir, s.spec.Version, "publish")
	if err != nil {
		return s.handleErr(fmt.Errorf("create shared cache publish marker failed: %w", err))
	}
	defer removeSharedCacheActiveMarker(markerFile)

	_ = os.RemoveAll(tempSnapshotDir)
	if err := os.MkdirAll(tempSnapshotDir, os.ModePerm); err != nil {
		return s.handleErr(fmt.Errorf("create temp snapshot dir failed: %w", err))
	}
	if err := copyDirContent(ctx, s.spec.CacheDir, tempSnapshotDir); err != nil {
		_ = os.RemoveAll(tempSnapshotDir)
		return s.handleErr(err)
	}
	if err := os.Rename(tempSnapshotDir, finalSnapshotDir); err != nil {
		_ = os.RemoveAll(tempSnapshotDir)
		return s.handleErr(fmt.Errorf("promote temp snapshot dir failed: %w", err))
	}
	snapshotPromoted = true

	leaseDuration := time.Duration(s.spec.LeaseDurationSeconds) * time.Second
	lock, err := lease.NewLock(s.spec.LeaseName, leaseDuration)
	if err != nil {
		return s.handleErr(fmt.Errorf("create shared cache publish lease lock failed: %w", err))
	}
	if err := lock.Acquire(ctx); err != nil {
		return s.handleErr(fmt.Errorf("acquire shared cache publish lease lock failed: %w", err))
	}
	lockReleased := false
	defer func() {
		if lockReleased {
			return
		}
		if err := lock.Release(context.Background()); err != nil {
			log.Errorf("release shared cache publish lease lock failed: %v", err)
		}
	}()

	restoreMeta, restoreMetaFound, err := loadSharedCacheRestoreMetadata(s.spec.MetadataFile)
	if err != nil {
		return s.handleErr(fmt.Errorf("load restore metadata failed: %w", err))
	}
	if !restoreMetaFound {
		log.Infof("Shared cache publish skipped current pointer update because restore metadata does not exist.")
		if err := lock.Release(context.Background()); err != nil {
			return s.handleErr(fmt.Errorf("release shared cache publish lease lock failed: %w", err))
		}
		lockReleased = true
		removeSharedCacheActiveMarker(markerFile)
		markerFile = ""
		if err := cleanupSharedCacheSnapshots(snapshotsDir, "", setting.SharedCacheSnapshotRetain); err != nil {
			return s.handleErr(fmt.Errorf("cleanup old shared cache snapshots failed: %w", err))
		}
		return nil
	}

	currentFile := filepath.Join(s.spec.StoreDir, "current.json")
	current, found, err := loadSharedCacheCurrent(currentFile)
	if err != nil {
		return s.handleErr(fmt.Errorf("load current cache metadata failed: %w", err))
	}
	if err := ensureSharedCachePublishLeaseHeld(lock); err != nil {
		return s.handleErr(err)
	}
	if stale, reason := sharedCacheBaseVersionStale(found, current, restoreMeta); stale {
		log.Infof("Shared cache publish skipped current pointer update because %s.", reason)
		protectedVersion := ""
		if found {
			protectedVersion = current.Version
		}
		if err := lock.Release(context.Background()); err != nil {
			return s.handleErr(fmt.Errorf("release shared cache publish lease lock failed: %w", err))
		}
		lockReleased = true
		removeSharedCacheActiveMarker(markerFile)
		markerFile = ""
		if err := cleanupSharedCacheSnapshots(snapshotsDir, protectedVersion, setting.SharedCacheSnapshotRetain); err != nil {
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
	if err := ensureSharedCachePublishLeaseHeld(lock); err != nil {
		return s.handleErr(err)
	}
	if err := writeSharedCacheCurrent(currentFile, current); err != nil {
		return s.handleErr(fmt.Errorf("write current cache metadata failed: %w", err))
	}
	currentUpdated = true
	if err := lock.Release(context.Background()); err != nil {
		return s.handleErr(fmt.Errorf("release shared cache publish lease lock failed: %w", err))
	}
	lockReleased = true
	removeSharedCacheActiveMarker(markerFile)
	markerFile = ""

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
		log.Errorf("shared cache publish failed, storeDir: %s, cacheDir: %s, metadataFile: %s, version: %s, leaseName: %s, err: %v",
			s.spec.StoreDir, s.spec.CacheDir, s.spec.MetadataFile, s.spec.Version, s.spec.LeaseName, err)
		return nil
	}
	return err
}

func ensureSharedCachePublishLeaseHeld(lock *lease.Lock) error {
	if err := lock.Check(); err != nil {
		return fmt.Errorf("shared cache publish lease lock lost before current pointer update: %w", err)
	}
	return nil
}

func sharedCacheBaseVersionStale(currentFound bool, current *sharedCacheCurrent, restoreMeta *sharedCacheRestoreMetadata) (bool, string) {
	if restoreMeta == nil {
		return true, "restore metadata is empty"
	}

	baseVersionFound := restoreMeta.BaseVersionFound || restoreMeta.BaseVersion != ""
	if !baseVersionFound {
		if currentFound && current != nil {
			return true, fmt.Sprintf("no base version was found when the task started, latest version is %s", current.Version)
		}
		return false, ""
	}

	if !currentFound || current == nil {
		return false, ""
	}
	if current.Version != restoreMeta.BaseVersion {
		return true, fmt.Sprintf("base version %s is stale, latest version is %s", restoreMeta.BaseVersion, current.Version)
	}
	return false, ""
}
