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
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/setting"
)

const (
	sharedCacheMarkerDirName = "markers"
	sharedCacheMarkerTTL     = 2 * time.Hour
	sharedCacheTempDirTTL    = 2 * time.Hour
)

var sharedCacheVersionNamePattern = regexp.MustCompile(`^task-\d+-[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}-[0-9a-fA-F]{16}$`)

type sharedCacheCurrent struct {
	Version         string `json:"version"`
	SnapshotDir     string `json:"snapshot_dir"`
	UpdatedAt       int64  `json:"updated_at"`
	UpdatedByTaskID int64  `json:"updated_by_task_id"`
	WorkflowName    string `json:"workflow_name"`
	JobName         string `json:"job_name"`
	BootstrapDir    string `json:"bootstrap_dir,omitempty"`
}

type sharedCacheRestoreMetadata struct {
	BaseVersion      string `json:"base_version"`
	BaseVersionFound bool   `json:"base_version_found"`
}

type sharedCacheActiveMarker struct {
	Version   string `json:"version"`
	Purpose   string `json:"purpose"`
	Holder    string `json:"holder"`
	CreatedAt int64  `json:"created_at"`
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

func writeSharedCacheRestoreMetadata(file, baseVersion string, baseVersionFound bool) error {
	meta := &sharedCacheRestoreMetadata{
		BaseVersion:      baseVersion,
		BaseVersionFound: baseVersionFound,
	}
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

func copyDirContent(ctx context.Context, src, dst string) error {
	return copyDirContentExclude(ctx, src, dst)
}

func copyDirContentExclude(ctx context.Context, src, dst string, excludes ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if nested, err := pathNested(src, dst); err != nil {
		return err
	} else if nested {
		return fmt.Errorf("copy destination %s must not be inside source %s", dst, src)
	}
	if err := os.MkdirAll(dst, os.ModePerm); err != nil {
		return err
	}
	excludeSet := make(map[string]struct{}, len(excludes))
	for _, exclude := range excludes {
		excludeSet[exclude] = struct{}{}
	}
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if _, ok := excludeSet[strings.Split(rel, string(os.PathSeparator))[0]]; ok {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := copyFile(path, target, info.Mode()); err != nil {
			return err
		}
		return os.Chtimes(target, info.ModTime(), info.ModTime())
	})
}

func pathNested(parent, child string) (bool, error) {
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return false, err
	}
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(parentAbs, childAbs)
	if err != nil {
		return false, err
	}
	return rel != "." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..", nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func dirHasContent(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for _, entry := range entries {
		if isSharedCacheInternalDir(entry.Name()) {
			continue
		}
		return true, nil
	}
	return false, nil
}

func removeDirContentExclude(dir string, excludes ...string) error {
	excludeSet := make(map[string]struct{}, len(excludes))
	for _, exclude := range excludes {
		excludeSet[exclude] = struct{}{}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if _, ok := excludeSet[entry.Name()]; ok {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func sharedCacheBootstrapDirLeaseName(bootstrapDir string) string {
	return "workflow-shared-cache-bootstrap-" + shortHash(filepath.Clean(bootstrapDir))
}

func sharedCacheInternalDirNames() []string {
	return []string{setting.SharedCacheStoreDataDir}
}

func isSharedCacheInternalDir(name string) bool {
	for _, internal := range sharedCacheInternalDirNames() {
		if name == internal {
			return true
		}
	}
	return false
}

func resolveSharedCacheSnapshotContentDir(snapshotDir, cacheDir string) (string, bool, error) {
	cacheDirName := filepath.Base(filepath.Clean(cacheDir))
	if cacheDirName == "" || cacheDirName == "." || cacheDirName == string(os.PathSeparator) {
		return snapshotDir, false, nil
	}
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		return "", false, err
	}
	if len(entries) != 1 || !entries[0].IsDir() || entries[0].Name() != cacheDirName {
		return snapshotDir, false, nil
	}
	return filepath.Join(snapshotDir, cacheDirName), true, nil
}

func sharedCacheSnapshotArtifactNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() && sharedCacheVersionNamePattern.MatchString(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

func createSharedCacheActiveMarker(storeDir, version, purpose string) (string, error) {
	if version == "" {
		return "", nil
	}
	holder, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("get hostname failed: %w", err)
	}
	holder = strings.TrimSpace(holder)
	if holder == "" {
		holder = "unknown"
	}

	marker := &sharedCacheActiveMarker{
		Version:   version,
		Purpose:   purpose,
		Holder:    holder,
		CreatedAt: time.Now().Unix(),
	}
	markerName := fmt.Sprintf("%d-%d-%s.json", time.Now().UnixNano(), os.Getpid(), shortHash(holder))
	markerFile := filepath.Join(storeDir, sharedCacheMarkerDirName, markerName)
	if err := writeJSONAtomic(markerFile, marker); err != nil {
		return "", err
	}
	return markerFile, nil
}

func removeSharedCacheActiveMarker(markerFile string) {
	if markerFile == "" {
		return
	}
	_ = os.Remove(markerFile)
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

	activeVersions, err := getSharedCacheActiveVersions(filepath.Join(filepath.Dir(snapshotsDir), sharedCacheMarkerDirName))
	if err != nil {
		return err
	}

	snapshots := make([]*sharedCacheSnapshot, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("get snapshot info %s failed: %w", name, err)
		}
		if strings.HasPrefix(name, ".tmp-") {
			if time.Since(info.ModTime()) > sharedCacheTempDirTTL {
				_ = os.RemoveAll(filepath.Join(snapshotsDir, name))
			}
			continue
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
	for version := range activeVersions {
		keep[version] = struct{}{}
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

func getSharedCacheActiveVersions(markersDir string) (map[string]struct{}, error) {
	activeVersions := make(map[string]struct{})
	entries, err := os.ReadDir(markersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return activeVersions, nil
		}
		return nil, err
	}

	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		markerFile := filepath.Join(markersDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("get active marker info %s failed: %w", entry.Name(), err)
		}
		if now.Sub(info.ModTime()) > sharedCacheMarkerTTL {
			_ = os.Remove(markerFile)
			continue
		}
		data, err := os.ReadFile(markerFile)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read active marker %s failed: %w", entry.Name(), err)
		}
		marker := &sharedCacheActiveMarker{}
		if err := json.Unmarshal(data, marker); err != nil {
			_ = os.Remove(markerFile)
			continue
		}
		if marker.Version == "" {
			_ = os.Remove(markerFile)
			continue
		}
		activeVersions[marker.Version] = struct{}{}
	}
	return activeVersions, nil
}

func shortHash(value string) string {
	hash := sha1.Sum([]byte(value))
	return hex.EncodeToString(hash[:8])
}
