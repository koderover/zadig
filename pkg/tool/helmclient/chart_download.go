package helmclient

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/repo"
)

func expandDownloadedChart(ctx context.Context, archive, dest string) error {
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

	if err := ctx.Err(); err != nil {
		return err
	}
	backup, err := replaceChartDirectory(filepath.Join(staging, ch.Name()), filepath.Join(dest, ch.Name()))
	if err != nil {
		return err
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("failed to clean old chart %s: %w", backup, err)
	}
	return nil
}

// Keep the previous directory until replacement succeeds, so errors can be rolled back.
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

// WithDownloadedChart holds the cache lock until the caller finishes using its files.
func (hClient *HelmClient) WithDownloadedChart(entry *repo.Entry, ref, version, dest string, use func(context.Context, string) error) error {
	return WithChartDirectoryLock(dest, func(ctx context.Context) error {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dest, chartCacheMarker), nil, 0644); err != nil {
			return err
		}
		name := filepath.Base(ref)
		chartPath := filepath.Join(dest, name)
		source := fmt.Sprintf("%x", sha256.Sum256([]byte(entry.URL+"\x00"+ref+"\x00"+version)))
		sourceFile := filepath.Join(dest, ".zadig-chart-"+name+".source")
		previous, err := os.ReadFile(sourceFile)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		hit := false
		if version != "" && string(previous) == source {
			cached, err := loader.Load(chartPath)
			hit = err == nil && cached.Metadata.Version == version
		}
		if !hit {
			// A failed refresh must never leave a marker that certifies an incomplete cache.
			if err := os.Remove(sourceFile); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := hClient.downloadChart(ctx, entry, ref, version, dest, true); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if use != nil {
			if err := use(ctx, chartPath); err != nil {
				return err
			}
		}
		return ctx.Err()
	})
}
