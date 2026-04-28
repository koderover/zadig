package step

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupSharedCacheSnapshotsKeepsActiveMarkerVersion(t *testing.T) {
	storeDir := t.TempDir()
	snapshotsDir := filepath.Join(storeDir, "snapshots")
	if err := os.MkdirAll(snapshotsDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	versions := []string{"v1", "v2", "v3"}
	for i, version := range versions {
		dir := filepath.Join(snapshotsDir, version)
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			t.Fatal(err)
		}
		modTime := time.Now().Add(time.Duration(i) * time.Second)
		if err := os.Chtimes(dir, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}

	markerFile, err := createSharedCacheActiveMarker(storeDir, "v1", "restore")
	if err != nil {
		t.Fatal(err)
	}
	defer removeSharedCacheActiveMarker(markerFile)

	if err := cleanupSharedCacheSnapshots(snapshotsDir, "v3", 1); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(snapshotsDir, "v1")); err != nil {
		t.Fatalf("active snapshot should be kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(snapshotsDir, "v2")); !os.IsNotExist(err) {
		t.Fatalf("unprotected old snapshot should be removed, err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(snapshotsDir, "v3")); err != nil {
		t.Fatalf("protected current snapshot should be kept: %v", err)
	}
}

func TestSharedCacheBaseVersionStaleForNoBaseVersion(t *testing.T) {
	stale, reason := sharedCacheBaseVersionStale(true, &sharedCacheCurrent{Version: "v1"}, &sharedCacheRestoreMetadata{
		BaseVersionFound: false,
	})
	if !stale {
		t.Fatalf("expected stale when no base version existed at task start but current exists now")
	}
	if reason == "" {
		t.Fatal("expected stale reason")
	}

	stale, reason = sharedCacheBaseVersionStale(false, nil, &sharedCacheRestoreMetadata{
		BaseVersionFound: false,
	})
	if stale {
		t.Fatalf("expected non-stale with no base version and no current, reason: %s", reason)
	}
}
