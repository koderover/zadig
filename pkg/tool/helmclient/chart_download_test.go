package helmclient

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otiai10/copy"
	"github.com/redis/go-redis/v9"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/repo"
)

func TestReplaceChartDirectory(t *testing.T) {
	for _, existing := range []string{"absent", "directory", "file"} {
		t.Run(existing, func(t *testing.T) {
			base := t.TempDir()
			source, target := filepath.Join(base, "new"), filepath.Join(base, "chart")
			if err := os.Mkdir(source, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "values.yaml"), []byte("replicas: 2\n"), 0644); err != nil {
				t.Fatal(err)
			}
			switch existing {
			case "directory":
				if err := os.Mkdir(target, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(target, "stale.yaml"), []byte("old"), 0644); err != nil {
					t.Fatal(err)
				}
			case "file":
				if err := os.WriteFile(target, []byte("old"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			backup, err := replaceChartDirectory(source, target)
			if err != nil {
				t.Fatal(err)
			}
			values, err := os.ReadFile(filepath.Join(target, "values.yaml"))
			if err != nil || string(values) != "replicas: 2\n" {
				t.Fatalf("unexpected values %q, err: %v", values, err)
			}
			if err := os.RemoveAll(backup); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(base)
			if err != nil || len(entries) != 1 || entries[0].Name() != "chart" {
				t.Fatalf("unexpected files after replacement: %v, err: %v", entries, err)
			}
		})
	}
}

func TestReplaceChartDirectoryRestoresPreviousOnFailure(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "chart")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	valuesPath := filepath.Join(target, "values.yaml")
	if err := os.WriteFile(valuesPath, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := replaceChartDirectory(filepath.Join(base, "missing"), target); err == nil {
		t.Fatal("expected publication to fail")
	}
	values, err := os.ReadFile(valuesPath)
	if err != nil || string(values) != "original" {
		t.Fatalf("old chart was not restored: %q, err: %v", values, err)
	}
	entries, err := os.ReadDir(base)
	if err != nil || len(entries) != 1 {
		t.Fatalf("backup was not cleaned: %v, err: %v", entries, err)
	}
}

func TestExpandDownloadedChartInvalidArchiveKeepsPrevious(t *testing.T) {
	base := t.TempDir()
	archive := filepath.Join(base, "invalid.tgz")
	if err := os.WriteFile(archive, []byte("invalid archive"), 0644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(base, "destination")
	if err := os.Mkdir(dest, 0755); err != nil {
		t.Fatal(err)
	}
	previous := filepath.Join(dest, "previous")
	if err := os.WriteFile(previous, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := expandDownloadedChart(context.Background(), archive, dest); err == nil {
		t.Fatal("expected invalid archive to fail")
	}
	data, err := os.ReadFile(previous)
	if err != nil || string(data) != "original" {
		t.Fatalf("old chart changed: %q, err: %v", data, err)
	}
}

func TestExpandDownloadedChartRejectsUnsafeName(t *testing.T) {
	for _, name := range []string{".", ".."} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			gz := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gz)
			metadata := []byte("apiVersion: v2\nname: " + name + "\nversion: 0.1.0\n")
			if err := tw.WriteHeader(&tar.Header{Name: "chart/Chart.yaml", Size: int64(len(metadata)), Mode: 0644}); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write(metadata); err != nil {
				t.Fatal(err)
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			if err := gz.Close(); err != nil {
				t.Fatal(err)
			}
			base := t.TempDir()
			archive := filepath.Join(base, "chart.tgz")
			if err := os.WriteFile(archive, buf.Bytes(), 0644); err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(base, "dest")
			if err := expandDownloadedChart(context.Background(), archive, dest); err == nil || !strings.Contains(err.Error(), "invalid downloaded chart name") {
				t.Fatalf("expected invalid chart name, got %v", err)
			}
			if _, err := os.Stat(dest); !os.IsNotExist(err) {
				t.Fatalf("invalid chart changed destination: %v", err)
			}
		})
	}
}

func TestChartCacheAndReaders(t *testing.T) {
	if os.Getenv("HELM_DOWNLOAD_TEST_REDIS") != "1" {
		t.Skip("requires a test Redis")
	}
	archiveDir := t.TempDir()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".tgz") {
			requests.Add(1)
		}
		http.FileServer(http.Dir(archiveDir)).ServeHTTP(w, r)
	}))
	defer server.Close()
	index := repo.NewIndexFile()
	for _, version := range []string{"1.0.0", "2.0.0"} {
		ch := &chart.Chart{
			Metadata: &chart.Metadata{APIVersion: "v2", Name: "collector", Version: version},
			Raw:      []*chart.File{{Name: "values.yaml", Data: []byte("replicas: 1\n")}},
		}
		archive, err := chartutil.Save(ch, archiveDir)
		if err != nil {
			t.Fatal(err)
		}
		if err := index.MustAdd(ch.Metadata, filepath.Base(archive), server.URL, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := index.WriteFile(filepath.Join(archiveDir, "index.yaml"), 0644); err != nil {
		t.Fatal(err)
	}
	oldSettings, oldInfo := generalSettings, repoInfo
	settings := cli.New()
	settings.RepositoryConfig = filepath.Join(t.TempDir(), "repositories.yaml")
	settings.RepositoryCache = t.TempDir()
	generalSettings, repoInfo = settings, &repo.File{}
	t.Cleanup(func() { generalSettings, repoInfo = oldSettings, oldInfo })
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	entry := &repo.Entry{Name: "test", URL: server.URL}
	dest := t.TempDir()
	chartPath := filepath.Join(dest, "collector")
	pull := func(version string) error {
		return client.DownloadChart(entry, "test/collector", version, dest, true)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := pull("1.0.0"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if t.Failed() {
		return
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("concurrent cache miss downloaded %d times, want 1", got)
	}
	if err := pull("1.0.0"); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("cache hit downloaded again: %d", got)
	}

	t.Run("reader blocks refresh", func(t *testing.T) {
		entered, release := make(chan struct{}), make(chan struct{})
		readDone := make(chan error, 1)
		go func() {
			readDone <- withChartReadLock(chartPath, func() error {
				close(entered)
				<-release
				ch, err := loader.Load(chartPath)
				if err == nil && ch.Metadata.Version != "1.0.0" {
					return fmt.Errorf("reader saw version %s", ch.Metadata.Version)
				}
				return err
			})
		}()
		<-entered
		writeDone := make(chan error, 1)
		go func() { writeDone <- pull("2.0.0") }()
		select {
		case err := <-writeDone:
			close(release)
			<-readDone
			t.Fatalf("refresh bypassed reader: %v", err)
		case <-time.After(150 * time.Millisecond):
		}
		close(release)
		if err := <-readDone; err != nil {
			t.Fatal(err)
		}
		if err := <-writeDone; err != nil {
			t.Fatal(err)
		}
		ch, err := loader.Load(chartPath)
		if err != nil || ch.Metadata.Version != "2.0.0" {
			t.Fatalf("refresh failed: %v, %v", ch, err)
		}
	})
	t.Run("invalid cache is downloaded again", func(t *testing.T) {
		before := requests.Load()
		if err := os.Remove(filepath.Join(chartPath, "Chart.yaml")); err != nil {
			t.Fatal(err)
		}
		if err := pull("2.0.0"); err != nil {
			t.Fatal(err)
		}
		if requests.Load() != before+1 {
			t.Fatal("incomplete cache was reused")
		}
	})
	t.Run("Helm loader waits for cache writer", func(t *testing.T) {
		err := WithChartDirectoryLock(dest, func(ctx context.Context) error {
			started := make(chan struct{})
			loaded := make(chan error, 1)
			go func() {
				close(started)
				_, _, err := client.getChart(chartPath, &action.ChartPathOptions{})
				loaded <- err
			}()
			<-started
			select {
			case err := <-loaded:
				return fmt.Errorf("Helm loader bypassed writer: %v", err)
			case <-time.After(150 * time.Millisecond):
			}
			// The loader resumes after this callback releases the writer lock.
			t.Cleanup(func() {
				if err := <-loaded; err != nil {
					t.Error(err)
				}
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("delivery edits preserve download cache", func(t *testing.T) {
		before := requests.Load()
		target := filepath.Join(t.TempDir(), "collector-2.0.0")
		for _, values := range []string{"replicas: 2\n", "replicas: 3\n"} {
			err := client.WithDownloadedChart(entry, "test/collector", "2.0.0", dest, func(ctx context.Context, source string) error {
				if err := os.RemoveAll(target); err != nil {
					return err
				}
				return copy.Copy(source, target)
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(target, "values.yaml"), []byte(values), 0644); err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(filepath.Join(chartPath, "values.yaml"))
			if err != nil || string(original) != "replicas: 1\n" {
				t.Fatalf("delivery changed source cache: %q, %v", original, err)
			}
		}
		if requests.Load() != before {
			t.Fatal("delivery downloaded an already cached chart")
		}
	})
	t.Run("repository identity is checked", func(t *testing.T) {
		before := requests.Load()
		alias := &repo.Entry{Name: "alias", URL: server.URL}
		if err := client.DownloadChart(alias, "alias/collector", "2.0.0", dest, true); err != nil {
			t.Fatal(err)
		}
		if requests.Load() != before+1 {
			t.Fatal("different chart reference reused stale cache")
		}
	})
	t.Run("cancelled extraction preserves old chart", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := expandDownloadedChart(ctx, filepath.Join(archiveDir, "collector-1.0.0.tgz"), dest)
		if err == nil {
			t.Fatal("cancelled publication succeeded")
		}
		ch, err := loader.Load(chartPath)
		if err != nil || ch.Metadata.Version != "2.0.0" {
			t.Fatalf("cancelled refresh changed cache: %v, %v", ch, err)
		}
	})
}

func TestChartDirectoryLockRenewalAndLoss(t *testing.T) {
	if os.Getenv("HELM_DOWNLOAD_TEST_REDIS") != "1" {
		t.Skip("requires a test Redis")
	}
	rdb := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT")})
	defer rdb.Close()
	dir := t.TempDir()
	key := "helm_chart_download:" + dir
	err := WithChartDirectoryLock(dir, func(ctx context.Context) error {
		time.Sleep(12 * time.Second)
		ttl, err := rdb.TTL(ctx, key).Result()
		if err != nil {
			return err
		}
		if ttl < 50*time.Second {
			return fmt.Errorf("lock was not renewed, remaining TTL: %s", ttl)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = WithChartDirectoryLock(dir, func(ctx context.Context) error {
		if err := rdb.Del(ctx, key).Err(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(15 * time.Second):
			return fmt.Errorf("lost lock did not cancel the operation")
		}
	})
	if err == nil || !strings.Contains(err.Error(), "lost chart directory lock") {
		t.Fatalf("expected lock-loss error, got %v", err)
	}
}
