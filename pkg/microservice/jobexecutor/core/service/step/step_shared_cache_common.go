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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type sharedCacheCurrent struct {
	Version         string `json:"version"`
	SnapshotDir     string `json:"snapshot_dir"`
	UpdatedAt       int64  `json:"updated_at"`
	UpdatedByTaskID int64  `json:"updated_by_task_id"`
	WorkflowName    string `json:"workflow_name"`
	JobName         string `json:"job_name"`
}

type sharedCacheRestoreMetadata struct {
	BaseVersion string `json:"base_version"`
}

func loadSharedCacheCurrent(file string) (*sharedCacheCurrent, bool, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	current := &sharedCacheCurrent{}
	if err := json.Unmarshal(data, current); err != nil {
		return nil, false, err
	}
	return current, true, nil
}

func loadSharedCacheRestoreMetadata(file string) (*sharedCacheRestoreMetadata, bool, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	meta := &sharedCacheRestoreMetadata{}
	if err := json.Unmarshal(data, meta); err != nil {
		return nil, false, err
	}
	return meta, true, nil
}

func writeSharedCacheRestoreMetadata(file, baseVersion string) error {
	meta := &sharedCacheRestoreMetadata{BaseVersion: baseVersion}
	return writeJSONAtomic(file, meta)
}

func writeSharedCacheCurrent(file string, current *sharedCacheCurrent) error {
	return writeJSONAtomic(file, current)
}

func writeJSONAtomic(file string, obj interface{}) error {
	if err := os.MkdirAll(filepath.Dir(file), os.ModePerm); err != nil {
		return err
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	tempFile := file + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tempFile, file)
}

func copyDirContent(src, dst string) error {
	if err := os.MkdirAll(dst, os.ModePerm); err != nil {
		return err
	}
	cmd := exec.Command("cp", "-a", filepath.Join(src, "."), dst)
	return cmd.Run()
}

type sharedCacheSnapshot struct {
	Name    string
	ModTime int64
}

func cleanupSharedCacheSnapshots(snapshotsDir, protectedVersion string, retain int) error {
	if retain <= 0 {
		return nil
	}
	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	snapshots := make([]*sharedCacheSnapshot, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".tmp-") {
			_ = os.RemoveAll(filepath.Join(snapshotsDir, name))
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("get snapshot info %s failed: %w", name, err)
		}
		snapshots = append(snapshots, &sharedCacheSnapshot{
			Name:    name,
			ModTime: info.ModTime().Unix(),
		})
	}

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].ModTime == snapshots[j].ModTime {
			return snapshots[i].Name > snapshots[j].Name
		}
		return snapshots[i].ModTime > snapshots[j].ModTime
	})

	keep := make(map[string]struct{}, retain+1)
	if protectedVersion != "" {
		keep[protectedVersion] = struct{}{}
	}
	recentKept := 0
	for _, snapshot := range snapshots {
		if recentKept >= retain {
			break
		}
		keep[snapshot.Name] = struct{}{}
		recentKept++
	}

	for _, snapshot := range snapshots {
		if _, ok := keep[snapshot.Name]; ok {
			continue
		}
		if err := os.RemoveAll(filepath.Join(snapshotsDir, snapshot.Name)); err != nil {
			return fmt.Errorf("remove old snapshot %s failed: %w", snapshot.Name, err)
		}
	}

	return nil
}
