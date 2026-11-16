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

package helmclient

import (
	"fmt"
	"io"
	"strings"
	"testing"

	hc "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/action"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
)

func TestHelmClientRateLimits(t *testing.T) {
	t.Setenv("HELM_DRIVER", "memory")
	for _, tt := range []struct {
		name      string
		qps       string
		burst     string
		wantQPS   float32
		wantBurst int
	}{
		{name: "defaults", wantQPS: 50, wantBurst: 300},
		{name: "configured", qps: "75", burst: "400", wantQPS: 75, wantBurst: 400},
		{name: "invalid", qps: "invalid", burst: "invalid", wantQPS: 50, wantBurst: 300},
		{name: "non-positive", qps: "0", burst: "-1", wantQPS: 50, wantBurst: 300},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HELM_CLIENT_QPS", tt.qps)
			t.Setenv("HELM_CLIENT_BURST", tt.burst)
			originalLimiter := flowcontrol.NewTokenBucketRateLimiter(1, 1)
			config := &rest.Config{
				Host:        "https://127.0.0.1:6443",
				QPS:         1,
				Burst:       1,
				RateLimiter: originalLimiter,
			}
			client, err := NewClientFromRestConf(config, "test")
			if err != nil {
				t.Fatal(err)
			}
			clone, err := client.Clone()
			if err != nil {
				t.Fatal(err)
			}
			if client.RestConfig.QPS != tt.wantQPS || client.RestConfig.Burst != tt.wantBurst {
				t.Fatalf("unexpected REST config rate limits: QPS=%v, Burst=%v", client.RestConfig.QPS, client.RestConfig.Burst)
			}
			for _, helmClient := range []*HelmClient{client, clone} {
				discoveryClient, err := helmClient.ActionConfig.RESTClientGetter.ToDiscoveryClient()
				if err != nil {
					t.Fatal(err)
				}
				limiter := discoveryClient.RESTClient().GetRateLimiter()
				if limiter.QPS() != tt.wantQPS {
					t.Fatalf("discovery QPS = %v, want %v", limiter.QPS(), tt.wantQPS)
				}
				// Exercise the actual discovery limiter, including the dependency's Burst override.
				for i := 0; i < tt.wantBurst; i++ {
					if !limiter.TryAccept() {
						t.Fatalf("discovery throttled after %d requests, want a burst of %d", i, tt.wantBurst)
					}
				}
			}
			if client.RestConfig == config || clone.RestConfig == client.RestConfig {
				t.Fatal("Helm clients must copy the REST config")
			}
			if config.QPS != 1 || config.Burst != 1 || config.RateLimiter != originalLimiter {
				t.Fatal("Helm initialization changed the original REST config")
			}
		})
	}
}

type historyDriver struct {
	*driver.Memory
	releases []*release.Release
	err      error
}

func (d *historyDriver) Query(map[string]string) ([]*release.Release, error) {
	return d.releases, d.err
}

func TestIsInstallOperation(t *testing.T) {
	tests := []struct {
		name        string
		releases    []*release.Release
		historyErr  error
		wantInstall bool
		wantReplace bool
		wantErr     string
	}{
		{
			name:        "release not found installs",
			historyErr:  driver.ErrReleaseNotFound,
			wantInstall: true,
		},
		{
			name:        "wrapped release not found installs",
			historyErr:  fmt.Errorf("query history: %w", driver.ErrReleaseNotFound),
			wantInstall: true,
		},
		{
			name:    "empty history is rejected",
			wantErr: `no revision for release "test-release"`,
		},
		{
			name:     "deployed release upgrades",
			releases: []*release.Release{newTestRelease(1, release.StatusDeployed)},
		},
		{
			name:     "pending release is rejected",
			releases: []*release.Release{newTestRelease(1, release.StatusPendingUpgrade)},
			wantErr:  "another operation (install/upgrade/rollback) is in progress",
		},
		{
			name:        "uninstalled release uses replace install",
			releases:    []*release.Release{newTestRelease(1, release.StatusUninstalled)},
			wantInstall: true,
			wantReplace: true,
		},
		{
			name:     "unknown release is rejected",
			releases: []*release.Release{newTestRelease(1, release.StatusUnknown)},
			wantErr:  "can't install or upgrade chart with status: unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hClient := newHistoryTestClient(&historyDriver{
				Memory:   driver.NewMemory(),
				releases: tt.releases,
				err:      tt.historyErr,
			})
			spec := &hc.ChartSpec{ReleaseName: "test-release", MaxHistory: 10}

			install, err := hClient.isInstallOperation(spec)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("isInstallOperation() unexpected error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("isInstallOperation() error = %v, want message containing %q", err, tt.wantErr)
			}
			if install != tt.wantInstall {
				t.Fatalf("isInstallOperation() install = %v, want %v", install, tt.wantInstall)
			}
			if spec.Replace != tt.wantReplace {
				t.Fatalf("isInstallOperation() Replace = %v, want %v", spec.Replace, tt.wantReplace)
			}
		})
	}
}

func newHistoryTestClient(history *historyDriver) *HelmClient {
	return &HelmClient{
		HelmClient: &hc.HelmClient{
			ActionConfig: &action.Configuration{
				KubeClient: &kubefake.PrintingKubeClient{Out: io.Discard},
				Releases:   storage.Init(history),
				Log:        func(string, ...interface{}) {},
			},
		},
	}
}

func newTestRelease(version int, status release.Status) *release.Release {
	return &release.Release{
		Version: version,
		Info:    &release.Info{Status: status},
	}
}
