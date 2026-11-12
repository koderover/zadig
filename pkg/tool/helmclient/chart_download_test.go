package helmclient

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"

	"github.com/koderover/zadig/v2/pkg/tool/cache"
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
	if err := expandDownloadedChart(archive, dest); err == nil {
		t.Fatal("expected invalid archive to fail")
	}
	data, err := os.ReadFile(previous)
	if err != nil || string(data) != "original" {
		t.Fatalf("old chart changed: %q, err: %v", data, err)
	}
}

// Run with HELM_DOWNLOAD_TEST_REDIS=1 and REDIS_HOST/REDIS_PORT pointing at a test Redis.
func TestRunPullConcurrentChartDownloads(t *testing.T) {
	if os.Getenv("HELM_DOWNLOAD_TEST_REDIS") != "1" {
		t.Skip("requires a test Redis")
	}
	archiveDir := t.TempDir()
	_, err := chartutil.Save(&chart.Chart{
		Metadata: &chart.Metadata{APIVersion: "v2", Name: "opentelemetry-collector", Version: "0.136.0"},
		Raw:      []*chart.File{{Name: "values.yaml", Data: []byte("replicas: 2\n")}},
	}, archiveDir)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.FileServer(http.Dir(archiveDir)))
	defer server.Close()
	settings := cli.New()
	settings.RepositoryConfig = filepath.Join(t.TempDir(), "repositories.yaml")
	settings.RepositoryCache = t.TempDir()
	dest := t.TempDir()
	chartDir := filepath.Join(dest, "opentelemetry-collector")
	if err := os.Mkdir(chartDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "stale.yaml"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				client := &HelmClient{}
				_, err := client.runPull(&action.Pull{
					Settings: settings, DestDir: dest, UntarDir: dest, Untar: true,
				}, server.URL+"/opentelemetry-collector-0.136.0.tgz")
				if err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
	}
	values, err := os.ReadFile(filepath.Join(chartDir, "values.yaml"))
	if err != nil || string(values) != "replicas: 2\n" {
		t.Fatalf("unexpected chart values: %q, err: %v", values, err)
	}
	if _, err := os.Stat(filepath.Join(chartDir, "stale.yaml")); !os.IsNotExist(err) {
		t.Fatalf("stale chart file remains: %v", err)
	}
	entries, err := os.ReadDir(dest)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary directories remain: %v, err: %v", entries, err)
	}

	t.Run("lock failure preserves chart", func(t *testing.T) {
		mutex := cache.NewRedisLockWithExpiry("helm_chart_download:"+dest, time.Minute)
		if err := mutex.Lock(); err != nil {
			t.Fatal(err)
		}
		defer mutex.Unlock()
		archive := filepath.Join(archiveDir, "opentelemetry-collector-0.136.0.tgz")
		if err := expandDownloadedChart(archive, dest); err == nil || !strings.Contains(err.Error(), "failed to lock chart destination") {
			t.Fatalf("expected lock acquisition failure, got %v", err)
		}
		values, err := os.ReadFile(filepath.Join(chartDir, "values.yaml"))
		if err != nil || string(values) != "replicas: 2\n" {
			t.Fatalf("chart changed after lock failure: %q, err: %v", values, err)
		}
		entries, err := os.ReadDir(dest)
		if err != nil || len(entries) != 1 {
			t.Fatalf("temporary directories remain after lock failure: %v, err: %v", entries, err)
		}
	})
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
			if err := expandDownloadedChart(archive, dest); err == nil || !strings.Contains(err.Error(), "invalid downloaded chart name") {
				t.Fatalf("expected invalid chart name, got %v", err)
			}
			if _, err := os.Stat(dest); !os.IsNotExist(err) {
				t.Fatalf("invalid chart changed destination: %v", err)
			}
		})
	}
}
