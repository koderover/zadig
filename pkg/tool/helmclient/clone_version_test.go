package helmclient

import (
	"path/filepath"
	"strings"
	"testing"

	hc "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/client-go/rest"
)

func TestHelmClientCloneRendersWithClusterVersion(t *testing.T) {
	t.Setenv("HELM_DRIVER", "memory")
	client, err := NewClientFromRestConf(&rest.Config{Host: "https://127.0.0.1:6443"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	client.KubeVersion, err = chartutil.ParseKubeVersion("v1.30.0")
	if err != nil {
		t.Fatal(err)
	}
	clone, err := client.Clone()
	if err != nil {
		t.Fatal(err)
	}

	chartDir := t.TempDir()
	err = chartutil.SaveDir(&chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion:  "v2",
			Name:        "version-check",
			Version:     "0.1.0",
			KubeVersion: ">=1.24.0-0",
		},
		Templates: []*chart.File{{
			Name: "templates/configmap.yaml",
			Data: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: version-check\ndata:\n  kubeVersion: {{ .Capabilities.KubeVersion.Version | quote }}\n"),
		}},
	}, chartDir)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := clone.TemplateChart(&hc.ChartSpec{
		ReleaseName: "version-check",
		ChartName:   filepath.Join(chartDir, "version-check"),
	}, &hc.HelmTemplateOptions{KubeVersion: clone.KubeVersion})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), `kubeVersion: "v1.30.0"`) {
		t.Fatalf("rendered manifest does not use the cluster version:\n%s", manifest)
	}
}
