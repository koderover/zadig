package helmclient

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/cache"
)

const chartCacheMarker = ".zadig-chart-cache"

// WithChartDirectoryLock protects a complete file operation, including readers.
// The callback must stop using the directory when its context is cancelled.
func WithChartDirectoryLock(dir string, use func(context.Context) error) (err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return err
	}
	mutex := cache.NewRedisLockWithExpiry("helm_chart_download:"+dir, time.Minute)
	if err := mutex.Lock(); err != nil {
		return fmt.Errorf("failed to lock chart destination %s: %w", dir, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				done <- nil
				return
			case <-ticker.C:
				renewCtx, stop := context.WithTimeout(ctx, 5*time.Second)
				err := mutex.ExtendContext(renewCtx)
				stop()
				if err != nil {
					if ctx.Err() != nil {
						done <- nil
					} else {
						cancel()
						done <- fmt.Errorf("lost chart directory lock %s: %w", dir, err)
					}
					return
				}
			}
		}
	}()
	defer func() {
		cancel()
		renewErr := <-done
		err = errors.Join(err, renewErr, mutex.Unlock())
	}()
	return use(ctx)
}

func withChartReadLock(chartPath string, read func() error) error {
	dir := filepath.Dir(chartPath)
	if _, err := os.Stat(filepath.Join(dir, chartCacheMarker)); os.IsNotExist(err) {
		return read()
	} else if err != nil {
		return err
	}
	return WithChartDirectoryLock(dir, func(ctx context.Context) error {
		if err := read(); err != nil {
			return err
		}
		return ctx.Err()
	})
}
