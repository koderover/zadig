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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage/driver"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"

	"github.com/koderover/zadig/lib/microservice/aslan/internal/log"
	"github.com/koderover/zadig/lib/tool/xlog"
)

var storage = repo.File{}

const (
	DefaultCachePath            = "/tmp/.helmcache"
	DefaultRepositoryConfigPath = "/tmp/.helmrepo"
)

// NewClientFromKubeConf returns a new Helm client constructed with the provided kubeconfig options
func NewClientFromKubeConf(options *KubeConfClientOptions) (Client, error) {
	settings := cli.New()
	if options.KubeConfig == nil {
		return nil, fmt.Errorf("kubeconfig missing")
	}

	clientGetter := NewRESTClientGetter(options.Namespace, options.KubeConfig, nil)
	err := setEnvSettings(options.Options, settings)
	if err != nil {
		return nil, err
	}

	if options.KubeContext != "" {
		settings.KubeContext = options.KubeContext
	}

	return newClient(options.Options, clientGetter, settings)
}

// NewClientFromRestConf returns a new Helm client constructed with the provided REST config options
func NewClientFromRestConf(restConfig *rest.Config, namespace string) (Client, error) {
	options := &RestConfClientOptions{
		Options: &Options{
			RepositoryCache:  DefaultCachePath,
			RepositoryConfig: DefaultRepositoryConfigPath,
			Debug:            true,
			Linting:          true,
			Namespace:        namespace,
		},
		RestConfig: restConfig,
	}

	settings := cli.New()
	clientGetter := NewRESTClientGetter(options.Namespace, nil, options.RestConfig)

	err := setEnvSettings(options.Options, settings)
	if err != nil {
		return nil, err
	}

	return newClient(options.Options, clientGetter, settings)
}

// newClient returns a new Helm client via the provided options and REST config
func newClient(options *Options, clientGetter genericclioptions.RESTClientGetter, settings *cli.EnvSettings) (Client, error) {
	err := setEnvSettings(options, settings)
	if err != nil {
		return nil, err
	}

	actionConfig := new(action.Configuration)
	err = actionConfig.Init(
		clientGetter,
		settings.Namespace(),
		os.Getenv("HELM_DRIVER"),
		func(format string, v ...interface{}) {
			log.Infof(format, v)
		},
	)
	if err != nil {
		return nil, err
	}

	return &HelmClient{
		Settings:     settings,
		Providers:    getter.All(settings),
		storage:      &storage,
		ActionConfig: actionConfig,
		linting:      options.Linting,
	}, nil
}

// setEnvSettings sets the client's environment settings based on the provided client configuration
func setEnvSettings(options *Options, settings *cli.EnvSettings) error {
	if options == nil {
		options = &Options{
			RepositoryConfig: DefaultRepositoryConfigPath,
			RepositoryCache:  DefaultCachePath,
			Linting:          true,
		}
	}

	// set the namespace with this ugly workaround because cli.EnvSettings.namespace is private
	// thank you helm!
	if options.Namespace != "" {
		pflags := pflag.NewFlagSet("", pflag.ContinueOnError)
		settings.AddFlags(pflags)
		err := pflags.Parse([]string{"-n", options.Namespace})
		if err != nil {
			return err
		}
	}

	if options.RepositoryConfig == "" {
		options.RepositoryConfig = DefaultRepositoryConfigPath
	}

	if options.RepositoryCache == "" {
		options.RepositoryCache = DefaultCachePath
	}

	settings.RepositoryCache = options.RepositoryCache
	settings.RepositoryConfig = DefaultRepositoryConfigPath
	settings.Debug = options.Debug

	return nil
}

// AddOrUpdateChartRepo adds or updates the provided helm chart repository
func (c *HelmClient) AddOrUpdateChartRepo(entry repo.Entry, isUpdate bool, log *xlog.Logger) error {
	chartRepo, err := repo.NewChartRepository(&entry, c.Providers)
	if err != nil {
		log.Errorf("chartRepo NewChartRepository err :%+v", err)
		return err
	}

	chartRepo.CachePath = c.Settings.RepositoryCache

	_, err = chartRepo.DownloadIndexFile()
	if err != nil {
		log.Errorf(fmt.Sprintf("chartRepo DownloadIndexFile err :%+v", err))
		return err
	}

	if c.storage.Has(entry.Name) && !isUpdate {
		log.Warnf("WARNING: repository name %q already exists", entry.Name)
		return nil
	}

	c.storage.Update(&entry)
	err = c.storage.WriteFile(c.Settings.RepositoryConfig, 0o644)
	if err != nil {
		log.Errorf("chartRepo WriteFile err :%+v", err)
		return err
	}

	return nil
}

// UpdateChartRepos updates the list of chart repositories stored in the client's cache
func (c *HelmClient) UpdateChartRepos(log *xlog.Logger) error {
	for _, entry := range c.storage.Repositories {
		chartRepo, err := repo.NewChartRepository(entry, c.Providers)
		if err != nil {
			return err
		}

		chartRepo.CachePath = c.Settings.RepositoryCache
		_, err = chartRepo.DownloadIndexFile()
		if err != nil {
			return err
		}

		c.storage.Update(entry)
	}

	return c.storage.WriteFile(c.Settings.RepositoryConfig, 0o644)
}

// InstallOrUpgradeChart triggers the installation of the provided chart.
// If the chart is already installed, trigger an upgrade instead
func (c *HelmClient) InstallOrUpgradeChart(ctx context.Context, spec *ChartSpec, chartOption *ChartOption, log *xlog.Logger) error {
	installed, err := c.chartIsInstalled(spec.ReleaseName)
	if err != nil {
		return err
	}
	if installed {
		return c.upgrade(ctx, spec, chartOption)
	}
	return c.install(spec, chartOption, log)
}

// DeleteChartFromCache deletes the provided chart from the client's cache
func (c *HelmClient) DeleteChartFromCache(spec *ChartSpec, chartOption *ChartOption) error {
	return c.deleteChartFromCache(spec, chartOption)
}

// UninstallRelease uninstalls the provided release
func (c *HelmClient) UninstallRelease(spec *ChartSpec) error {
	return c.uninstallRelease(spec)
}

// install lints and installs the provided chart
func (c *HelmClient) install(spec *ChartSpec, chartOption *ChartOption, log *xlog.Logger) error {
	client := action.NewInstall(c.ActionConfig)
	mergeInstallOptions(spec, client)

	if client.Version == "" {
		client.Version = ">0.0.0-0"
	}

	helmChart, chartPath, err := c.getChart(spec.ChartName, &client.ChartPathOptions, chartOption)
	if err != nil {
		return err
	}

	if helmChart.Metadata.Type != "" && helmChart.Metadata.Type != "application" {
		return fmt.Errorf(
			"chart %q has an unsupported type and is not installable: %q",
			helmChart.Metadata.Name,
			helmChart.Metadata.Type,
		)
	}

	if req := helmChart.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(helmChart, req); err != nil {
			if client.DependencyUpdate {
				man := &downloader.Manager{
					ChartPath:        chartPath,
					Keyring:          client.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          c.Providers,
					RepositoryConfig: c.Settings.RepositoryConfig,
					RepositoryCache:  c.Settings.RepositoryCache,
				}
				if err := man.Update(); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	values, err := spec.GetValuesMap()
	if err != nil {
		return err
	}

	if c.linting {
		err = c.lint(chartPath, values)
		if err != nil {
			return err
		}
	}

	_, err = client.Run(helmChart, values)
	if err != nil {
		log.Errorf("release installed: %s/%s-%s err: %+v", spec.ChartName, spec.Namespace, spec.Version, err)
		return err
	}

	return nil
}

// upgrade upgrades a chart and CRDs
func (c *HelmClient) upgrade(ctx context.Context, spec *ChartSpec, chartOption *ChartOption) error {
	client := action.NewUpgrade(c.ActionConfig)
	mergeUpgradeOptions(spec, client)

	if client.Version == "" {
		client.Version = ">0.0.0-0"
	}

	helmChart, chartPath, err := c.getChart(spec.ChartName, &client.ChartPathOptions, chartOption)
	if err != nil {
		return err
	}

	if req := helmChart.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(helmChart, req); err != nil {
			return err
		}
	}

	values, err := spec.GetValuesMap()
	if err != nil {
		return err
	}

	if c.linting {
		err = c.lint(chartPath, values)
		if err != nil {
			return err
		}
	}

	if !spec.SkipCRDs && spec.UpgradeCRDs {
		err = c.upgradeCRDs(ctx, helmChart)
		if err != nil {
			return err
		}
	}

	_, err = client.Run(spec.ReleaseName, helmChart, values)
	if err != nil {
		log.Errorf("release upgrade: %s/%s-%s err: %+v", spec.ChartName, spec.Namespace, spec.Version, err)
		return err
	}
	return nil
}

// deleteChartFromCache deletes the provided chart from the client's cache
func (c *HelmClient) deleteChartFromCache(spec *ChartSpec, chartOption *ChartOption) error {
	client := action.NewChartRemove(c.ActionConfig)

	helmChart, _, err := c.getChart(spec.ChartName, &action.ChartPathOptions{}, chartOption)
	if err != nil {
		log.Errorf("deleteChartFromCache getChart err:%+v", err)
		return err
	}

	var deleteOutputBuffer bytes.Buffer
	err = client.Run(&deleteOutputBuffer, helmChart.Name())
	if err != nil {
		log.Errorf("deleteChartFromCache chart removed: %s/%s-%s err: %+v", helmChart.Name(), spec.ReleaseName, helmChart.AppVersion(), err)
		return err
	}

	return nil
}

// uninstallRelease uninstalls the provided release
func (c *HelmClient) uninstallRelease(spec *ChartSpec) error {
	client := action.NewUninstall(c.ActionConfig)
	mergeUninstallReleaseOptions(spec, client)

	resp, err := client.Run(spec.ReleaseName)
	if err != nil {
		log.Errorf("release removed, response: %v, err: %+v", resp, err)
		return err
	}

	return nil
}

// lint lints a chart's values
func (c *HelmClient) lint(chartPath string, values map[string]interface{}) error {
	client := action.NewLint()

	result := client.Run([]string{chartPath}, values)

	for _, err := range result.Errors {
		log.Errorf("Error: %s", err)
	}

	if len(result.Errors) > 0 {
		return fmt.Errorf("linting for chartpath %q failed", chartPath)
	}

	return nil
}

// chart template
func (c *HelmClient) TemplateChart(spec *ChartSpec, chartOption *ChartOption, log *xlog.Logger) ([]byte, error) {
	client := action.NewInstall(c.ActionConfig)
	mergeInstallOptions(spec, client)

	client.DryRun = true
	client.ReleaseName = spec.ReleaseName
	client.Replace = true // Skip the name check
	client.ClientOnly = true
	client.APIVersions = []string{}
	client.IncludeCRDs = true

	if client.Version == "" {
		client.Version = ">0.0.0-0"
	}

	helmChart, chartPath, err := c.getChart(spec.ChartName, &client.ChartPathOptions, chartOption)
	if err != nil {
		return nil, err
	}

	if helmChart.Metadata.Type != "" && helmChart.Metadata.Type != "application" {
		return nil, fmt.Errorf(
			"chart %q has an unsupported type and is not installable: %q",
			helmChart.Metadata.Name,
			helmChart.Metadata.Type,
		)
	}

	if req := helmChart.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(helmChart, req); err != nil {
			if client.DependencyUpdate {
				man := &downloader.Manager{
					ChartPath:        chartPath,
					Keyring:          client.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          c.Providers,
					RepositoryConfig: c.Settings.RepositoryConfig,
					RepositoryCache:  c.Settings.RepositoryCache,
				}
				if err := man.Update(); err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}

	values, err := spec.GetValuesMap()
	if err != nil {
		return nil, err
	}

	out := new(bytes.Buffer)
	rel, err := client.Run(helmChart, values)
	if err != nil {
		log.Errorf("TemplateChart run err: %+v", err)
	}

	if rel != nil {
		var manifests bytes.Buffer
		fmt.Fprintf(&manifests, strings.TrimSpace(rel.Manifest))
		if !client.DisableHooks {
			for _, m := range rel.Hooks {
				fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", m.Path, m.Manifest)
			}
		}

		fmt.Fprintf(out, "%s", manifests.String())
	}

	return out.Bytes(), err
}

// 检测chart信息
func (c *HelmClient) LintChart(spec *ChartSpec, chartOption *ChartOption, log *xlog.Logger) error {
	_, chartPath, err := c.getChart(spec.ChartName, &action.ChartPathOptions{}, chartOption)
	if err != nil {
		return err
	}

	values, err := spec.GetValuesMap()
	if err != nil {
		return err
	}

	return c.lint(chartPath, values)
}

// upgradeCRDs
func (c *HelmClient) upgradeCRDs(ctx context.Context, chartInstance *chart.Chart) error {
	cfg, err := c.Settings.RESTClientGetter().ToRESTConfig()
	if err != nil {
		return err
	}
	k8sClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		return err
	}

	for _, crd := range chartInstance.CRDObjects() {
		jsonCRD, err := yaml.ToJSON(crd.File.Data)
		if err != nil {
			return err
		}

		var meta metav1.TypeMeta
		err = json.Unmarshal(jsonCRD, &meta)
		if err != nil {
			return err
		}

		switch meta.APIVersion {
		case "apiextensions.k8s.io/v1":
			var crdObj v1.CustomResourceDefinition
			err = json.Unmarshal(jsonCRD, &crdObj)
			if err != nil {
				return err
			}
			existingCRDObj, err := k8sClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdObj.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			crdObj.ResourceVersion = existingCRDObj.ResourceVersion
			_, err = k8sClient.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, &crdObj, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

		case "apiextensions.k8s.io/v1beta1":
			var crdObj v1beta1.CustomResourceDefinition
			err = json.Unmarshal(jsonCRD, &crdObj)
			if err != nil {
				return err
			}
			existingCRDObj, err := k8sClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(ctx, crdObj.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			crdObj.ResourceVersion = existingCRDObj.ResourceVersion
			_, err = k8sClient.ApiextensionsV1beta1().CustomResourceDefinitions().Update(ctx, &crdObj, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("failed to update crd %q: unsupported api-version %q", crd.Name, meta.APIVersion)
		}
	}

	return nil
}

// getChart returns a chart matching the provided chart name and options
func (c *HelmClient) getChart(chartName string, chartPathOptions *action.ChartPathOptions, chartOption *ChartOption) (*chart.Chart, string, error) {
	var (
		chartPath = ""
		err       error
	)
	if chartOption == nil {
		chartPath, err = chartPathOptions.LocateChart(chartName, c.Settings)
	} else {
		chartPath = chartOption.ChartPath
	}
	if err != nil {
		return nil, "", err
	}

	helmChart, err := loader.Load(chartPath)
	if err != nil {
		return nil, "", err
	}

	if helmChart.Metadata.Deprecated {
		log.Infof("WARNING: This chart (%q) is deprecated", helmChart.Metadata.Name)
	}

	return helmChart, chartPath, err
}

// chartIsInstalled checks whether a chart is already installed or not by the provided release name
func (c *HelmClient) chartIsInstalled(release string) (bool, error) {
	histClient := action.NewHistory(c.ActionConfig)
	histClient.Max = 1
	if _, err := histClient.Run(release); err == driver.ErrReleaseNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// mergeInstallOptions merges values of the provided chart to helm install options used by the client
func mergeInstallOptions(chartSpec *ChartSpec, installOptions *action.Install) {
	installOptions.DisableHooks = chartSpec.DisableHooks
	installOptions.Replace = chartSpec.Replace
	installOptions.Wait = chartSpec.Wait
	installOptions.DependencyUpdate = chartSpec.DependencyUpdate
	installOptions.Timeout = chartSpec.Timeout
	installOptions.Namespace = chartSpec.Namespace
	installOptions.ReleaseName = chartSpec.ReleaseName
	installOptions.Version = chartSpec.Version
	installOptions.GenerateName = chartSpec.GenerateName
	installOptions.NameTemplate = chartSpec.NameTemplate
	installOptions.Atomic = chartSpec.Atomic
	installOptions.SkipCRDs = chartSpec.SkipCRDs
	installOptions.SubNotes = chartSpec.SubNotes
}

// mergeUpgradeOptions merges values of the provided chart to helm upgrade options used by the client
func mergeUpgradeOptions(chartSpec *ChartSpec, upgradeOptions *action.Upgrade) {
	upgradeOptions.Version = chartSpec.Version
	upgradeOptions.Namespace = chartSpec.Namespace
	upgradeOptions.Timeout = chartSpec.Timeout
	upgradeOptions.Wait = chartSpec.Wait
	upgradeOptions.DisableHooks = chartSpec.DisableHooks
	upgradeOptions.Force = chartSpec.Force
	upgradeOptions.ResetValues = chartSpec.ResetValues
	upgradeOptions.ReuseValues = chartSpec.ReuseValues
	upgradeOptions.Recreate = chartSpec.Recreate
	upgradeOptions.MaxHistory = chartSpec.MaxHistory
	upgradeOptions.Atomic = chartSpec.Atomic
	upgradeOptions.CleanupOnFail = chartSpec.CleanupOnFail
	upgradeOptions.SubNotes = chartSpec.SubNotes
}

// mergeUninstallReleaseOptions merges values of the provided chart to helm uninstall options used by the client
func mergeUninstallReleaseOptions(chartSpec *ChartSpec, uninstallReleaseOptions *action.Uninstall) {
	uninstallReleaseOptions.DisableHooks = chartSpec.DisableHooks
	uninstallReleaseOptions.Timeout = chartSpec.Timeout
}
