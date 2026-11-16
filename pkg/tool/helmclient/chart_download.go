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

	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func expandDownloadedChart(ctx context.Context, archive, dest string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
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
		return fmt.Errorf("failed to create chart staging directory in %s: %w", dest, err)
	}
	defer func() {
		if err := os.RemoveAll(staging); err != nil {
			log.Warnf("Failed to clean chart staging directory %s: %v", staging, err)
		}
	}()
	if err := chartutil.ExpandFile(staging, archive); err != nil {
		return fmt.Errorf("failed to untar chart: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	return replaceChartDirectory(filepath.Join(staging, ch.Name()), filepath.Join(dest, ch.Name()))
}

// Keep the previous directory until replacement succeeds, so errors can be rolled back.
func replaceChartDirectory(source, target string) error {
	backup, err := os.MkdirTemp(filepath.Dir(target), ".helm-backup-")
	if err != nil {
		return fmt.Errorf("failed to create backup directory for chart %s: %w", target, err)
	}
	previous := filepath.Join(backup, "chart")
	err = os.Rename(target, previous)
	if err != nil && !os.IsNotExist(err) {
		_ = os.RemoveAll(backup)
		return fmt.Errorf("failed to move old chart %s: %w", target, err)
	}
	hadPrevious := err == nil
	if err := os.Rename(source, target); err != nil {
		if hadPrevious {
			if restoreErr := os.Rename(previous, target); restoreErr != nil {
				return fmt.Errorf("failed to publish chart: %w; failed to restore old chart from %s: %v", err, previous, restoreErr)
			}
		}
		_ = os.RemoveAll(backup)
		return fmt.Errorf("failed to publish chart %s: %w", target, err)
	}
	if err := os.RemoveAll(backup); err != nil {
		log.Warnf("Failed to clean old chart backup %s: %v", backup, err)
	}
	return nil
}

// WithDownloadedChart holds the cache lock until the caller finishes using its files.
func (hClient *HelmClient) WithDownloadedChart(entry *repo.Entry, ref, version, dest string, use func(context.Context, string) error) error {
	return WithChartDirectoryLock(dest, func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := filepath.Base(ref)
		chartPath := filepath.Join(dest, name)
		if filepath.Base(filepath.Clean(dest)) == "latest" {
			if err := os.RemoveAll(dest); err != nil {
				return fmt.Errorf("failed to clear latest chart %s: %w", dest, err)
			}
			if err := hClient.downloadChart(ctx, entry, ref, version, dest, true); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(dest, 0755); err != nil {
				return fmt.Errorf("failed to create chart cache directory %s: %w", dest, err)
			}
			source := fmt.Sprintf("%x", sha256.Sum256([]byte(entry.URL+"\x00"+ref+"\x00"+version)))
			sourceFile := filepath.Join(dest, ".zadig-chart-"+name+".source")
			previous, err := os.ReadFile(sourceFile)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to read chart cache source marker %s: %w", sourceFile, err)
			}
			hit := false
			if version != "" && string(previous) == source {
				cached, err := loader.Load(chartPath)
				hit = err == nil && cached.Metadata.Version == version
			}
			if !hit {
				// An empty marker keeps readers locked while the cache is being refreshed.
				if err := os.WriteFile(sourceFile, nil, 0644); err != nil {
					return fmt.Errorf("failed to mark chart cache refresh pending in %s: %w", sourceFile, err)
				}
				if err := hClient.downloadChart(ctx, entry, ref, version, dest, true); err != nil {
					return err
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
					return fmt.Errorf("failed to write chart cache source marker %s: %w", sourceFile, err)
				}
			}
		}

		if err := ctx.Err(); err != nil {
			return err
		}
		if use != nil {
			return use(ctx, chartPath)
		}
		return nil
	})
}

// WithChartDirectoryLock protects a complete file operation, including readers.
// The callback must stop using the directory when its context is cancelled.
func WithChartDirectoryLock(dir string, use func(context.Context) error) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	return cache.WithRedisLock("helm_chart_download:"+dir, use)
}

func withChartReadLock(chartPath string, read func() error) error {
	dir := filepath.Dir(chartPath)
	if filepath.Base(filepath.Clean(dir)) != "latest" {
		sourceFile := filepath.Join(dir, ".zadig-chart-"+filepath.Base(chartPath)+".source")
		if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
			return read()
		} else if err != nil {
			return err
		}
	}
	return WithChartDirectoryLock(dir, func(context.Context) error { return read() })
}
