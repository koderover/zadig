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

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/tool/kube/lease"
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
		log.Infof("Start recording shared cache base version from store dir %s for cache dir %s without restoring cache content.", s.spec.StoreDir, s.spec.CacheDir)
	} else {
		log.Infof("Start restoring shared cache from store dir %s to cache dir %s.", s.spec.StoreDir, s.spec.CacheDir)
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
		if !s.spec.SkipContent {
			bootstrapped, version, err := s.bootstrapFromExistingCacheDir(ctx, currentFile)
			if err != nil {
				return s.handleErr(fmt.Errorf("bootstrap shared cache from existing cache dir failed: %w", err))
			}
			if bootstrapped {
				current, found, err = loadSharedCacheCurrent(currentFile)
				if err != nil {
					return s.handleErr(fmt.Errorf("reload current cache metadata failed: %w", err))
				}
				if !found {
					return s.handleErr(fmt.Errorf("shared cache bootstrap finished with version %s but current metadata is still missing", version))
				}
				log.Infof("Shared cache initialized from existing cache dir %s with version %s.", s.spec.CacheDir, version)
			} else {
				log.Infof("Shared cache restore skipped because current cache metadata does not exist.")
				return s.handleErr(writeSharedCacheRestoreMetadata(s.spec.MetadataFile, "", false))
			}
		} else {
			log.Infof("Shared cache restore skipped because current cache metadata does not exist.")
			return s.handleErr(writeSharedCacheRestoreMetadata(s.spec.MetadataFile, "", false))
		}
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
	if s.spec.SkipContent {
		if err := writeSharedCacheRestoreMetadata(s.spec.MetadataFile, current.Version, true); err != nil {
			return s.handleErr(fmt.Errorf("write restore metadata failed: %w", err))
		}
		log.Infof("Shared cache content restore skipped with base version %s.", current.Version)
		return nil
	}

	restoreDir, legacyWrapped, err := resolveSharedCacheSnapshotContentDir(snapshotDir, s.spec.CacheDir)
	if err != nil {
		return s.handleErr(fmt.Errorf("resolve snapshot content dir failed: %w", err))
	}
	if legacyWrapped {
		log.Infof("Shared cache restore detected legacy wrapped snapshot layout, restoring content from %s.", restoreDir)
	}
	if err := copyDirContent(ctx, restoreDir, s.spec.CacheDir); err != nil {
		return s.handleErr(err)
	}
	if err := writeSharedCacheRestoreMetadata(s.spec.MetadataFile, current.Version, true); err != nil {
		return s.handleErr(fmt.Errorf("write restore metadata failed: %w", err))
	}
	log.Infof("Shared cache restore finished with version %s.", current.Version)
	return nil
}

func (s *SharedCacheRestoreStep) bootstrapFromExistingCacheDir(ctx context.Context, currentFile string) (bool, string, error) {
	if s.spec.LeaseName == "" || s.spec.Version == "" {
		log.Infof("Shared cache bootstrap skipped because lease name or version is empty.")
		return false, "", nil
	}
	bootstrapDir := s.spec.BootstrapDir
	if bootstrapDir == "" {
		bootstrapDir = s.spec.CacheDir
	}
	if hasContent, err := dirHasContent(bootstrapDir); err != nil {
		return false, "", err
	} else if !hasContent {
		log.Infof("Shared cache bootstrap skipped because bootstrap dir %s does not exist or is empty.", bootstrapDir)
		return false, "", nil
	}

	leaseDuration := time.Duration(s.spec.LeaseDurationSeconds) * time.Second
	bootstrapDirLock, err := lease.NewLock(sharedCacheBootstrapDirLeaseName(bootstrapDir), leaseDuration)
	if err != nil {
		return false, "", fmt.Errorf("create shared cache bootstrap dir lease lock failed: %w", err)
	}
	if err := bootstrapDirLock.Acquire(ctx); err != nil {
		return false, "", fmt.Errorf("acquire shared cache bootstrap dir lease lock failed: %w", err)
	}
	bootstrapDirLockReleased := false
	defer func() {
		if bootstrapDirLockReleased {
			return
		}
		if err := bootstrapDirLock.Release(context.Background()); err != nil {
			log.Errorf("release shared cache bootstrap dir lease lock failed: %v", err)
		}
	}()
	if hasContent, err := dirHasContent(bootstrapDir); err != nil {
		return false, "", err
	} else if !hasContent {
		log.Infof("Shared cache bootstrap skipped because bootstrap dir %s does not exist or is empty after acquiring lock.", bootstrapDir)
		return false, "", nil
	}

	lock, err := lease.NewLock(s.spec.LeaseName, leaseDuration)
	if err != nil {
		return false, "", fmt.Errorf("create shared cache bootstrap lease lock failed: %w", err)
	}
	if err := lock.Acquire(ctx); err != nil {
		return false, "", fmt.Errorf("acquire shared cache bootstrap lease lock failed: %w", err)
	}
	lockReleased := false
	defer func() {
		if lockReleased {
			return
		}
		if err := lock.Release(context.Background()); err != nil {
			log.Errorf("release shared cache bootstrap lease lock failed: %v", err)
		}
	}()

	current, found, err := loadSharedCacheCurrent(currentFile)
	if err != nil {
		return false, "", fmt.Errorf("reload current cache metadata failed: %w", err)
	}
	if found && current != nil && current.Version != "" {
		if err := lock.Release(context.Background()); err != nil {
			return false, "", fmt.Errorf("release shared cache bootstrap lease lock failed: %w", err)
		}
		lockReleased = true
		if err := bootstrapDirLock.Release(context.Background()); err != nil {
			return false, "", fmt.Errorf("release shared cache bootstrap dir lease lock failed: %w", err)
		}
		bootstrapDirLockReleased = true
		log.Infof("Shared cache bootstrap skipped because current cache metadata was initialized with version %s.", current.Version)
		return true, current.Version, nil
	}

	if err := ensureSharedCachePublishLeaseHeld(lock); err != nil {
		return false, "", err
	}
	if err := ensureSharedCachePublishLeaseHeld(bootstrapDirLock); err != nil {
		return false, "", err
	}
	snapshotsDir := filepath.Join(s.spec.StoreDir, "snapshots")
	tempSnapshotDir := filepath.Join(snapshotsDir, ".tmp-"+s.spec.Version)
	finalSnapshotDir := filepath.Join(snapshotsDir, s.spec.Version)
	snapshotPromoted := false
	currentUpdated := false
	defer func() {
		if snapshotPromoted && !currentUpdated {
			if err := os.RemoveAll(finalSnapshotDir); err != nil {
				log.Errorf("remove unpublished shared cache bootstrap snapshot %s failed: %v", finalSnapshotDir, err)
			}
		}
	}()
	if err := os.MkdirAll(snapshotsDir, os.ModePerm); err != nil {
		return false, "", fmt.Errorf("create snapshot dir failed: %w", err)
	}
	_ = os.RemoveAll(tempSnapshotDir)
	if err := os.MkdirAll(tempSnapshotDir, os.ModePerm); err != nil {
		return false, "", fmt.Errorf("create temp snapshot dir failed: %w", err)
	}
	if err := copyDirContentExclude(ctx, bootstrapDir, tempSnapshotDir, sharedCacheInternalDirNames()...); err != nil {
		_ = os.RemoveAll(tempSnapshotDir)
		return false, "", err
	}
	if err := os.Rename(tempSnapshotDir, finalSnapshotDir); err != nil {
		_ = os.RemoveAll(tempSnapshotDir)
		return false, "", fmt.Errorf("promote temp snapshot dir failed: %w", err)
	}
	snapshotPromoted = true
	if err := ensureSharedCachePublishLeaseHeld(lock); err != nil {
		return false, "", err
	}
	if err := ensureSharedCachePublishLeaseHeld(bootstrapDirLock); err != nil {
		return false, "", err
	}
	current = &sharedCacheCurrent{
		Version:         s.spec.Version,
		SnapshotDir:     filepath.Join("snapshots", s.spec.Version),
		UpdatedAt:       time.Now().Unix(),
		UpdatedByTaskID: s.spec.TaskID,
		WorkflowName:    s.spec.WorkflowName,
		JobName:         s.spec.JobName,
		BootstrapDir:    bootstrapDir,
	}
	if err := writeSharedCacheCurrent(currentFile, current); err != nil {
		return false, "", fmt.Errorf("write current cache metadata failed: %w", err)
	}
	currentUpdated = true
	if err := lock.Release(context.Background()); err != nil {
		return false, "", fmt.Errorf("release shared cache bootstrap lease lock failed: %w", err)
	}
	lockReleased = true
	if err := bootstrapDirLock.Release(context.Background()); err != nil {
		return false, "", fmt.Errorf("release shared cache bootstrap dir lease lock failed: %w", err)
	}
	bootstrapDirLockReleased = true
	return true, s.spec.Version, nil
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
