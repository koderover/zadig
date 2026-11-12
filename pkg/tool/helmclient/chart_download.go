package helmclient

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/koderover/zadig/v2/pkg/tool/cache"
)

func expandDownloadedChart(archive, dest string) error {
	dest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	ch, err := loader.LoadFile(archive)
	if err != nil {
		return fmt.Errorf("failed to load downloaded chart: %w", err)
	}
	if ch.Name() == "." || ch.Name() == ".." {
		return fmt.Errorf("invalid downloaded chart name %q", ch.Name())
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("failed to create chart destination: %w", err)
	}
	staging, err := os.MkdirTemp(dest, ".helm-download-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := chartutil.ExpandFile(staging, archive); err != nil {
		return fmt.Errorf("failed to untar chart: %w", err)
	}

	// Network I/O and extraction happen before locking, so slow downloads cannot outlive the lease.
	mutex := cache.NewRedisLockWithExpiry(fmt.Sprintf("helm_chart_download:%s", dest), time.Minute)
	if err := mutex.Lock(); err != nil {
		return fmt.Errorf("failed to lock chart destination %s: %w", dest, err)
	}
	backup, err := replaceChartDirectory(filepath.Join(staging, ch.Name()), filepath.Join(dest, ch.Name()))
	unlockErr := mutex.Unlock()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("failed to clean old chart %s: %w", backup, err)
	}
	if unlockErr != nil {
		return fmt.Errorf("failed to unlock chart destination %s: %w", dest, unlockErr)
	}
	return nil
}

// Return the backup for cleanup after releasing the destination lock.
func replaceChartDirectory(source, target string) (string, error) {
	backup, err := os.MkdirTemp(filepath.Dir(target), ".helm-backup-")
	if err != nil {
		return "", err
	}
	previous := filepath.Join(backup, "chart")
	err = os.Rename(target, previous)
	if err != nil && !os.IsNotExist(err) {
		_ = os.RemoveAll(backup)
		return "", fmt.Errorf("failed to move old chart %s: %w", target, err)
	}
	hadPrevious := err == nil
	if err := os.Rename(source, target); err != nil {
		if hadPrevious {
			if restoreErr := os.Rename(previous, target); restoreErr != nil {
				return backup, fmt.Errorf("failed to publish chart: %w; failed to restore old chart from %s: %v", err, previous, restoreErr)
			}
		}
		_ = os.RemoveAll(backup)
		return "", fmt.Errorf("failed to publish chart %s: %w", target, err)
	}
	return backup, nil
}
