/*
Copyright 2021 The KodeRover Authors.

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
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/client-go/rest"
)

// KubeConfClientOptions defines the options used for constructing a client via kubeconfig
type KubeConfClientOptions struct {
	*Options
	KubeContext string
	KubeConfig  []byte
}

// RestConfClientOptions defines the options used for constructing a client via REST config
type RestConfClientOptions struct {
	*Options
	RestConfig *rest.Config
}

// Options defines the options of a client
type Options struct {
	Namespace        string
	RepositoryConfig string
	RepositoryCache  string
	Debug            bool
	Linting          bool
}

// RESTClientGetter defines the values of a helm REST client
type RESTClientGetter struct {
	namespace  string
	kubeConfig []byte
	restConfig *rest.Config
}

// Client defines the values of a helm client
type HelmClient struct {
	Settings     *cli.EnvSettings
	Providers    getter.Providers
	storage      *repo.File
	ActionConfig *action.Configuration
	linting      bool
}

// ChartSpec defines the values of a helm chart
type ChartSpec struct {
	ReleaseName string `json:"release"`
	ChartName   string `json:"chart"`
	Namespace   string `json:"namespace"`

	// use string instead of map[string]interface{}
	// https://github.com/kubernetes-sigs/kubebuilder/issues/528#issuecomment-466449483
	// and https://github.com/kubernetes-sigs/controller-tools/pull/317
	// +optional
	ValuesYaml string `json:"valuesYaml,omitempty"`

	// +optional
	Version string `json:"version,omitempty"`

	// +optional
	DisableHooks bool `json:"disableHooks,omitempty"`

	// +optional
	Replace bool `json:"replace,omitempty"`

	// +optional
	Wait bool `json:"wait,omitempty"`

	// +optional
	DependencyUpdate bool `json:"dependencyUpdate,omitempty"`

	// +optional
	Timeout time.Duration `json:"timeout,omitempty"`

	// +optional
	GenerateName bool `json:"generateName,omitempty"`

	// +optional
	NameTemplate string `json:"NameTemplate,omitempty"`

	// +optional
	Atomic bool `json:"atomic,omitempty"`

	// +optional
	SkipCRDs bool `json:"skipCRDs,omitempty"`

	// +optional
	UpgradeCRDs bool `json:"upgradeCRDs,omitempty"`

	// +optional
	SubNotes bool `json:"subNotes,omitempty"`

	// +optional
	Force bool `json:"force,omitempty"`

	// +optional
	ResetValues bool `json:"resetValues,omitempty"`

	// +optional
	ReuseValues bool `json:"reuseValues,omitempty"`

	// +optional
	Recreate bool `json:"recreate,omitempty"`

	// +optional
	MaxHistory int `json:"maxHistory,omitempty"`

	// +optional
	CleanupOnFail bool `json:"cleanupOnFail,omitempty"`
}

type ChartOption struct {
	ChartPath string `json:"chart_path"`
}
