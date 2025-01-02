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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	cm "github.com/chartmuseum/helm-push/pkg/chartmuseum"
	hc "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/plugin"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"helm.sh/helm/v3/pkg/strvals"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
	yamlutil "github.com/koderover/zadig/v2/pkg/util/yaml"
)

const (
	HelmPluginsDirectory = "/app/.helm/helmplugin"
)

var repoInfo *repo.File
var generalSettings *cli.EnvSettings

// enable support of oci registry
func init() {
	_ = os.Setenv("HELM_EXPERIMENTAL_OCI", "1")
	_ = os.Setenv("HELM_PLUGINS", HelmPluginsDirectory)
	repoInfo = &repo.File{}
	generalSettings = cli.New()
	generalSettings.PluginsDirectory = HelmPluginsDirectory
}

type HelmClient struct {
	*hc.HelmClient
	kubeClient client.Client
	Namespace  string
	lock       *sync.Mutex
	RestConfig *rest.Config
}

// NewClient returns a new Helm client with no construct parameters
// used to update helm repo data and download index.yaml or helm charts
func NewClient() (*HelmClient, error) {
	hcClient, err := hc.New(&hc.Options{
		RepositoryConfig: generalSettings.RepositoryConfig,
		RepositoryCache:  generalSettings.RepositoryCache,
	})

	if err != nil {
		return nil, err
	}
	helmClient := hcClient.(*hc.HelmClient)
	helmClient.Settings = generalSettings
	return &HelmClient{
		helmClient,
		nil,
		"",
		&sync.Mutex{},
		nil,
	}, nil
}

// NewClientFromNamespace returns a new Helm client constructed with the provided clusterID and namespace
// a kubeClient will be initialized to support necessary k8s operations when install/upgrade helm charts
func NewClientFromNamespace(clusterID, namespace string) (*HelmClient, error) {
	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return nil, err
	}

	hcClient, err := hc.NewClientFromRestConf(&hc.RestConfClientOptions{
		Options: &hc.Options{
			Namespace: namespace,
			DebugLog:  log.Debugf,
		},
		RestConfig: restConfig,
	})
	if err != nil {
		return nil, err
	}

	helmClient := hcClient.(*hc.HelmClient)
	return &HelmClient{
		helmClient,
		kubeClient,
		namespace,
		&sync.Mutex{},
		restConfig,
	}, nil
}

// NewClientFromRestConf returns a new Helm client constructed with the provided REST config options
// only used to list/uninstall helm release because kubeClient is nil
func NewClientFromRestConf(restConfig *rest.Config, ns string) (*HelmClient, error) {
	hcClient, err := hc.NewClientFromRestConf(&hc.RestConfClientOptions{
		Options: &hc.Options{
			Namespace: ns,
			DebugLog:  log.Debugf,
		},
		RestConfig: restConfig,
	})
	if err != nil {
		return nil, err
	}

	helmClient := hcClient.(*hc.HelmClient)
	return &HelmClient{
		helmClient,
		nil,
		ns,
		&sync.Mutex{},
		restConfig,
	}, nil
}

type KV struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// @note when should set valuesYaml? for return all values of the chart?
// MergeOverrideValues merge override yaml and override kvs
// defaultValues overrideYaml used for -f option
// overrideValues used for --set option
func MergeOverrideValues(valuesYaml, defaultValues, overrideYaml, overrideValues string, imageKvs []*KV) (string, error) {

	// merge files for helm -f option
	// precedence from low to high: images valuesYaml defaultValues overrideYaml

	var imageRelatedValues []byte
	if len(imageKvs) > 0 {
		imageValuesMap := make(map[string]interface{})
		imageKvStr := make([]string, 0)
		// image related values
		for _, imageKv := range imageKvs {
			imageKvStr = append(imageKvStr, fmt.Sprintf("%s=%v", imageKv.Key, imageKv.Value))
		}
		err := strvals.ParseInto(strings.Join(imageKvStr, ","), imageValuesMap)
		if err != nil {
			return "", err
		}
		imageRelatedValues, err = yaml.Marshal(imageValuesMap)
		if err != nil {
			return "", err
		}
	}

	valuesMap, err := yamlutil.MergeAndUnmarshal([][]byte{[]byte(valuesYaml), []byte(defaultValues), []byte(overrideYaml), imageRelatedValues})
	if err != nil {
		return "", err
	}

	kvStr := make([]string, 0)
	// merge kv values for helm --set option
	if overrideValues != "" {
		kvList := make([]*KV, 0)
		err = json.Unmarshal([]byte(overrideValues), &kvList)
		if err != nil {
			return "", err
		}
		for _, kv := range kvList {
			kvStr = append(kvStr, fmt.Sprintf("%s=%v", kv.Key, kv.Value))
		}
	}

	//// image related values
	//for _, imageKv := range imageKvs {
	//	kvStr = append(kvStr, fmt.Sprintf("%s=%v", imageKv.Key, imageKv.Value))
	//}
	//

	// override values for --set option
	if len(kvStr) > 0 {
		err = strvals.ParseInto(strings.Join(kvStr, ","), valuesMap)
		if err != nil {
			return "", err
		}
	}

	bs, err := yaml.Marshal(valuesMap)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// upgradeCRDs upgrades the CRDs of the provided chart.
func (hClient *HelmClient) upgradeCRDs(ctx context.Context, chartInstance *chart.Chart) error {
	cfg, err := hClient.ActionConfig.RESTClientGetter.ToRESTConfig()
	if err != nil {
		return err
	}

	k8sClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		return err
	}

	for _, crd := range chartInstance.CRDObjects() {
		if err := hClient.upgradeCRD(ctx, k8sClient, crd); err != nil {
			return err
		}
		hClient.DebugLog("CRD %s upgraded successfully for chart: %s", crd.Name, chartInstance.Metadata.Name)
	}

	return nil
}

// upgradeCRDV1Beta1 upgrades a CRD of the v1 API version using the provided k8s client and CRD yaml.
func (hClient *HelmClient) upgradeCRDV1(ctx context.Context, cl *clientset.Clientset, rawCRD []byte) error {
	var crdObj v1.CustomResourceDefinition
	if err := yaml.Unmarshal(rawCRD, &crdObj); err != nil {
		return err
	}

	existingCRDObj, err := cl.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdObj.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Check to ensure that no previously existing API version is deleted through the upgrade.
	if len(existingCRDObj.Spec.Versions) > len(crdObj.Spec.Versions) {
		hClient.DebugLog("WARNING: new version of CRD %q would remove an existing API version, skipping upgrade", crdObj.Name)
		return nil
	}

	// Check that the storage version does not change through the update.
	oldStorageVersion := v1.CustomResourceDefinitionVersion{}

	for _, oldVersion := range existingCRDObj.Spec.Versions {
		if oldVersion.Storage {
			oldStorageVersion = oldVersion
		}
	}

	i := 0

	for _, newVersion := range crdObj.Spec.Versions {
		if newVersion.Storage {
			i++
			if newVersion.Name != oldStorageVersion.Name {
				return fmt.Errorf("ERROR: storage version of CRD %q changed, aborting upgrade", crdObj.Name)
			}
		}
		if i > 1 {
			return fmt.Errorf("ERROR: more than one storage version set on CRD %q, aborting upgrade", crdObj.Name)
		}
	}

	if reflect.DeepEqual(existingCRDObj.Spec.Versions, crdObj.Spec.Versions) {
		hClient.DebugLog("INFO: new version of CRD %q contains no changes, skipping upgrade", crdObj.Name)
		return nil
	}

	crdObj.ResourceVersion = existingCRDObj.ResourceVersion
	if _, err := cl.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, &crdObj, metav1.UpdateOptions{DryRun: []string{"All"}}); err != nil {
		return err
	}
	hClient.DebugLog("upgrade ran successful for CRD (dry run): %s", crdObj.Name)

	if _, err := cl.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, &crdObj, metav1.UpdateOptions{}); err != nil {
		return err
	}
	hClient.DebugLog("upgrade ran successful for CRD: %s", crdObj.Name)

	return nil
}

// upgradeCRDV1Beta1 upgrades a CRD of the v1beta1 API version using the provided k8s client and CRD yaml.
func (hClient *HelmClient) upgradeCRDV1Beta1(ctx context.Context, cl *clientset.Clientset, rawCRD []byte) error {
	var crdObj v1beta1.CustomResourceDefinition
	if err := yaml.Unmarshal(rawCRD, &crdObj); err != nil {
		return err
	}
	existingCRDObj, err := cl.ApiextensionsV1beta1().CustomResourceDefinitions().Get(ctx, crdObj.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Check that the storage version does not change through the update.
	oldStorageVersion := v1beta1.CustomResourceDefinitionVersion{}

	for _, oldVersion := range existingCRDObj.Spec.Versions {
		if oldVersion.Storage {
			oldStorageVersion = oldVersion
		}
	}

	i := 0

	for _, newVersion := range crdObj.Spec.Versions {
		if newVersion.Storage {
			i++
			if newVersion.Name != oldStorageVersion.Name {
				return fmt.Errorf("ERROR: storage version of CRD %q changed, aborting upgrade", crdObj.Name)
			}
		}
		if i > 1 {
			return fmt.Errorf("ERROR: more than one storage version set on CRD %q, aborting upgrade", crdObj.Name)
		}
	}

	if reflect.DeepEqual(existingCRDObj.Spec.Versions, crdObj.Spec.Versions) {
		hClient.DebugLog("INFO: new version of CRD %q contains no changes, skipping upgrade", crdObj.Name)
		return nil
	}

	crdObj.ResourceVersion = existingCRDObj.ResourceVersion
	if _, err := cl.ApiextensionsV1beta1().CustomResourceDefinitions().Update(ctx, &crdObj, metav1.UpdateOptions{DryRun: []string{"All"}}); err != nil {
		return err
	}
	hClient.DebugLog("upgrade ran successful for CRD (dry run): %s", crdObj.Name)

	if _, err = cl.ApiextensionsV1beta1().CustomResourceDefinitions().Update(ctx, &crdObj, metav1.UpdateOptions{}); err != nil {
		return err
	}
	hClient.DebugLog("upgrade ran successful for CRD: %s", crdObj.Name)

	return nil
}

// upgradeCRD upgrades the CRD 'crd' using the provided k8s client.
func (hClient *HelmClient) upgradeCRD(ctx context.Context, k8sClient *clientset.Clientset, crd chart.CRD) error {
	var typeMeta metav1.TypeMeta
	err := yaml.Unmarshal(crd.File.Data, &typeMeta)
	if err != nil {
		return err
	}

	switch typeMeta.APIVersion {
	case "apiextensions.k8s.io/v1beta1":
		return hClient.upgradeCRDV1Beta1(ctx, k8sClient, crd.File.Data)
	case "apiextensions.k8s.io/v1":
		return hClient.upgradeCRDV1(ctx, k8sClient, crd.File.Data)
	default:
		return fmt.Errorf("WARNING: failed to upgrade CRD %q: unsupported api-version %q", crd.Name, typeMeta.APIVersion)
	}
}

// check weather to install or upgrade chart by current status
// return error if neither install nor upgrade action is legal
func (hClient *HelmClient) isInstallOperation(spec *hc.ChartSpec) (bool, error) {
	historyReleaseCount := 10
	if spec.MaxHistory > 0 {
		historyReleaseCount = spec.MaxHistory
	}
	// find history of particular release
	releases, err := hClient.ListReleaseHistory(spec.ReleaseName, historyReleaseCount)
	if err != nil && err != driver.ErrReleaseNotFound {
		return false, err
	}
	// release not found, install operation
	if len(releases) == 0 {
		return true, nil
	}

	releaseutil.Reverse(releases, releaseutil.SortByRevision)
	lastRelease := releases[0]

	// pending status
	if lastRelease.Info.Status.IsPending() {
		return false, errors.New("another operation (install/upgrade/rollback) is in progress, please try later")
	}

	// find deployed revision with status deployed from history, would be upgrade operation
	for _, rel := range releases {
		if rel.Info.Status == release.StatusDeployed {
			return false, nil
		}
	}

	// release with failed/superseded status: legal upgrade operation
	if lastRelease.Info.Status == release.StatusFailed || lastRelease.Info.Status == release.StatusSuperseded {
		return false, hClient.ensureUpgrade(historyReleaseCount, spec.ReleaseName, releases)
	}

	// if replace set to true, install will be a legal operation
	if st := lastRelease.Info.Status; spec.Replace && (st == release.StatusUninstalled || st == release.StatusFailed) {
		return true, nil
	}

	return false, fmt.Errorf("can't install or upgrade chart with status: %s", lastRelease.Info.Status)
}

// ensure new release revision can be saved
func (hClient *HelmClient) ensureUpgrade(maxHistoryCount int, releaseName string, releases []*release.Release) error {
	if maxHistoryCount <= 0 || len(releases) < maxHistoryCount {
		return nil
	}
	if hClient.kubeClient == nil {
		return errors.New("kubeClient is nil")
	}
	secretName := fmt.Sprintf("%s.%s.v%d", storage.HelmStorageType, releaseName, releases[len(releases)-1].Version)
	return updater.DeleteSecretWithName(hClient.Namespace, secretName, hClient.kubeClient)
}

// getChart returns a chart matching the provided chart name and options.
func (hClient *HelmClient) getChart(chartName string, chartPathOptions *action.ChartPathOptions) (*chart.Chart, string, error) {
	chartPath, err := chartPathOptions.LocateChart(chartName, hClient.HelmClient.Settings)
	if err != nil {
		return nil, "", err
	}

	helmChart, err := loader.Load(chartPath)
	if err != nil {
		return nil, "", err
	}

	if helmChart.Metadata.Deprecated {
		hClient.HelmClient.DebugLog("WARNING: This chart (%q) is deprecated", helmChart.Metadata.Name)
	}

	return helmChart, chartPath, err
}

func (hClient *HelmClient) installChart(ctx context.Context, spec *hc.ChartSpec) (*release.Release, error) {
	c := hClient.HelmClient
	install := action.NewInstall(c.ActionConfig)
	mergeInstallOptions(spec, install)

	if install.Version == "" {
		install.Version = ">0.0.0-0"
	}

	helmChart, chartPath, err := hClient.getChart(spec.ChartName, &install.ChartPathOptions)
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
			if !install.DependencyUpdate {
				return nil, err
			}
			man := &downloader.Manager{
				ChartPath:        chartPath,
				Keyring:          install.ChartPathOptions.Keyring,
				SkipUpdate:       false,
				Getters:          c.Providers,
				RepositoryConfig: generalSettings.RepositoryConfig,
				RepositoryCache:  generalSettings.RepositoryCache,
			}
			if err := man.Update(); err != nil {
				return nil, err
			}
		}
	}

	values, err := spec.GetValuesMap()
	if err != nil {
		return nil, err
	}

	rel, err := install.RunWithContext(ctx, helmChart, values)
	if err != nil {
		return rel, err
	}

	c.DebugLog("release installed successfully: %s/%s-%s", rel.Name, rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)

	return rel, nil
}

func (hClient *HelmClient) upgradeChart(ctx context.Context, spec *hc.ChartSpec) (*release.Release, error) {
	c := hClient.HelmClient
	upgrade := action.NewUpgrade(c.ActionConfig)
	mergeUpgradeOptions(spec, upgrade)

	if upgrade.Version == "" {
		upgrade.Version = ">0.0.0-0"
	}

	helmChart, _, err := hClient.getChart(spec.ChartName, &upgrade.ChartPathOptions)
	if err != nil {
		return nil, err
	}

	if req := helmChart.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(helmChart, req); err != nil {
			return nil, err
		}
	}

	values, err := spec.GetValuesMap()
	if err != nil {
		return nil, err
	}

	if !spec.SkipCRDs && spec.UpgradeCRDs {
		c.DebugLog("upgrading crds")
		err = hClient.upgradeCRDs(ctx, helmChart)
		if err != nil {
			return nil, err
		}
	}

	rel, err := upgrade.RunWithContext(ctx, spec.ReleaseName, helmChart, values)
	if err != nil {
		return rel, err
	}
	c.DebugLog("release upgraded successfully: %s/%s-%s", rel.Name, rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)
	return rel, nil
}

// InstallOrUpgradeChart install or upgrade helm chart, use the same rule with helm to determine weather to install or upgrade
func (hClient *HelmClient) InstallOrUpgradeChart(ctx context.Context, spec *hc.ChartSpec, opts *hc.GenericHelmOptions) (*release.Release, error) {
	install, err := hClient.isInstallOperation(spec)
	if err != nil {
		return nil, err
	}

	if install {
		return hClient.installChart(ctx, spec)
	} else {
		return hClient.upgradeChart(ctx, spec)
	}
}

// UpdateChartRepo works like executing `helm repo update`
// environment `HELM_REPO_USERNAME` and `HELM_REPO_PASSWORD` are only required for ali acr repos
func (hClient *HelmClient) UpdateChartRepo(repoEntry *repo.Entry) (string, error) {
	chartRepo, err := repo.NewChartRepository(repoEntry, hClient.Providers)
	if err != nil {
		return "", err
	}
	chartRepo.CachePath = hClient.Settings.RepositoryCache

	repoUrl, err := url.Parse(repoEntry.URL)
	if err != nil {
		return "", fmt.Errorf("failed to parse repo url: %s, err: %w", repoEntry.URL, err)
	}
	if repoUrl.Scheme == "acr" {
		// export envionment-variables for ali acr chart repo
		_ = os.Setenv("HELM_REPO_USERNAME", repoEntry.Username)
		_ = os.Setenv("HELM_REPO_PASSWORD", repoEntry.Password)
	}

	// update repo info
	repoInfo.Update(repoEntry)

	// download index.yaml
	indexFilePath, err := chartRepo.DownloadIndexFile()
	if err != nil {
		return "", err
	}

	err = repoInfo.WriteFile(hClient.Settings.RepositoryConfig, 0o644)
	if err != nil {
		return "", err
	}
	return indexFilePath, err
}

// FetchIndexYaml fetch index.yaml from remote chart repo
// `helm repo add` and `helm repo update` will be executed
func (hClient *HelmClient) FetchIndexYaml(repoEntry *repo.Entry) (*repo.IndexFile, error) {
	hClient.lock.Lock()
	defer hClient.lock.Unlock()

	if registry.IsOCI(repoEntry.URL) {
		return &repo.IndexFile{
			Entries: make(map[string]repo.ChartVersions),
		}, nil
	}

	indexFilePath, err := hClient.UpdateChartRepo(repoEntry)
	if err != nil {
		return nil, err
	}
	// Read the index file for the repository to get chart information and return chart URL
	repoIndex, err := repo.LoadIndexFile(indexFilePath)
	return repoIndex, err
}

// DownloadChart works like executing `helm pull repoName/chartName --version=version'
// since pulling from OCI Registry is still considered as an EXPERIMENTAL feature
// we DO NOT support pulling charts by pulling OCI Artifacts from OCI Registry
// NOTE consider using os.execCommand('helm pull') to reduce code complexity of offering compatibility since third-party plugins CANNOT be used as SDK
// if unTar is true, no need to mkdir for destDir
// if unTar is no, your need to mkdir for destDir yourself
func (hClient *HelmClient) DownloadChart(repoEntry *repo.Entry, chartRef string, chartVersion string, destDir string, unTar bool) error {
	hClient.lock.Lock()
	defer hClient.lock.Unlock()

	// download chart from ocr registry
	if registry.IsOCI(repoEntry.URL) {
		log.Infof("start download chart from oci registry, chartRef: %s", chartRef)
		chartNameStr := strings.Split(chartRef, "/")
		if len(chartNameStr) < 2 {
			return fmt.Errorf("chart name is not valid")
		}
		chartRef = fmt.Sprintf("%s/%s", repoEntry.URL, chartNameStr[len(chartNameStr)-1])
		return hClient.downloadOCIChart(repoEntry, chartRef, chartVersion, destDir, unTar)
	}

	_, err := hClient.UpdateChartRepo(repoEntry)
	if err != nil {
		return err
	}
	pull := action.NewPullWithOpts(action.WithConfig(&action.Configuration{}))
	pull.Password = repoEntry.Username
	pull.Username = repoEntry.Password
	pull.Version = chartVersion
	pull.Settings = generalSettings
	pull.DestDir = destDir
	pull.UntarDir = destDir
	pull.Untar = unTar
	_, err = pull.Run(chartRef)
	return err
}

func (hClient *HelmClient) downloadOCIChart(repoEntry *repo.Entry, chartRef string, chartVersion string, destDir string, unTar bool) error {
	pullConfig := &action.Configuration{}
	var err error
	pullConfig.RegistryClient, err = registry.NewClient(
		registry.ClientOptEnableCache(true),
		registry.ClientOptDebug(true),
		registry.ClientOptWriter(os.Stdout),
	)
	if err != nil {
		return err
	}
	hostUrl := strings.TrimPrefix(repoEntry.URL, fmt.Sprintf("%s://", registry.OCIScheme))
	err = pullConfig.RegistryClient.Login(hostUrl, registry.LoginOptBasicAuth(repoEntry.Username, repoEntry.Password))
	if err != nil {
		return err
	}
	pull := action.NewPullWithOpts(action.WithConfig(pullConfig))
	pull.Password = repoEntry.Username
	pull.Username = repoEntry.Password
	pull.Version = chartVersion
	pull.Settings = generalSettings
	pull.DestDir = destDir
	pull.UntarDir = destDir
	pull.Untar = unTar
	_, err = pull.Run(chartRef)
	return err
}

func (hClient *HelmClient) pushAcrChart(repoEntry *repo.Entry, chartPath string) error {
	base := filepath.Join(hClient.Settings.PluginsDirectory, "helm-acr")
	prog := exec.Command(filepath.Join(base, "bin/helm-cm-push"), chartPath, repoEntry.Name)
	plugin.SetupPluginEnv(hClient.Settings, "cm-push", base)
	prog.Env = os.Environ()
	buf := bytes.NewBuffer(nil)
	prog.Stdout = buf
	prog.Stderr = buf
	if err := prog.Run(); err != nil {
		if eErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("plugin exited with error: %s %s", string(eErr.Stderr), buf.String())
		}
		return fmt.Errorf("%s %s", err, buf.String())
	}
	return nil
}

func (hClient *HelmClient) pushChartMuseum(repoEntry *repo.Entry, chartPath string) error {
	chartClient, err := cm.NewClient(
		cm.URL(repoEntry.URL),
		cm.Username(repoEntry.Username),
		cm.Password(repoEntry.Password),
	)
	if err != nil {
		return err
	}
	resp, err := chartClient.UploadChartPackage(chartPath, false)
	if err != nil {
		return fmt.Errorf("failed to prepare pushing chart: %s, error: %w", chartPath, err)

	}

	defer resp.Body.Close()
	err = handlePushResponse(resp)
	if err != nil {
		return fmt.Errorf("failed to push chart: %s, error: %w", chartPath, err)
	}
	return nil
}

func (hClient *HelmClient) pushOCIRegistry(repoEntry *repo.Entry, chartPath string) error {
	pushConfig := &action.Configuration{}
	var err error
	pushConfig.RegistryClient, err = registry.NewClient(
		registry.ClientOptEnableCache(true),
		registry.ClientOptDebug(true),
		registry.ClientOptWriter(os.Stdout),
	)

	hostUrl := strings.TrimPrefix(repoEntry.URL, fmt.Sprintf("%s://", registry.OCIScheme))
	err = pushConfig.RegistryClient.Login(hostUrl, registry.LoginOptBasicAuth(repoEntry.Username, repoEntry.Password))
	if err != nil {
		return err
	}

	push := action.NewPushWithOpts(action.WithPushConfig(pushConfig))
	push.Settings = generalSettings
	_, err = push.Run(chartPath, repoEntry.URL)
	return err
}

func getChartmuseumError(b []byte, code int) error {
	var er struct {
		Error string `json:"error"`
	}
	err := json.Unmarshal(b, &er)
	if err != nil || er.Error == "" {
		return fmt.Errorf("%d: could not properly parse response JSON: %s", code, string(b))
	}
	return fmt.Errorf("chart museum errCode: %d, err: %s", code, er.Error)
}

func handlePushResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return getChartmuseumError(b, resp.StatusCode)
	}
	log.Info("push chart to chart repo done")
	return nil
}

func (hClient *HelmClient) PushChart(repoEntry *repo.Entry, chartPath string) error {
	hClient.lock.Lock()
	defer hClient.lock.Unlock()
	repoUrl, err := url.Parse(repoEntry.URL)
	if err != nil {
		return fmt.Errorf("failed to parse repo url: %s, err: %w", repoEntry.URL, err)
	}
	if repoUrl.Scheme == "acr" {
		return hClient.pushAcrChart(repoEntry, chartPath)
	} else if repoUrl.Scheme == registry.OCIScheme {
		return hClient.pushOCIRegistry(repoEntry, chartPath)
	} else {
		_, err := hClient.UpdateChartRepo(repoEntry)
		if err != nil {
			return err
		}
		return hClient.pushChartMuseum(repoEntry, chartPath)
	}
}

func (hClient *HelmClient) GetChartValues(repoEntry *repo.Entry, projectName, releaseName, chartRepo, chartName, chartVersion string, isProduction bool) (string, error) {
	chartRef := fmt.Sprintf("%s/%s", chartRepo, chartName)
	localPath := config.LocalServicePathWithRevision(projectName, releaseName, chartVersion, isProduction)

	lock := cache.NewRedisLock(fmt.Sprintf("download-chart-%s", localPath))
	lock.Lock()
	defer lock.Unlock()

	// remove local file to untar
	_ = os.RemoveAll(localPath)

	err := hClient.DownloadChart(repoEntry, chartRef, chartVersion, localPath, true)
	if err != nil {
		return "", fmt.Errorf("failed to download chart, chartName: %s, chartRepo: %+v, err: %s", chartName, repoEntry.Name, err)
	}

	fsTree := os.DirFS(localPath)
	valuesYAML, err := util.ReadValuesYAML(fsTree, chartName, log.SugaredLogger())
	if err != nil {
		return "", err
	}

	return string(valuesYAML), nil
}

// NOTE: When using this method, pay attention to whether restConfig is present in the original client.
func (hClient *HelmClient) Clone() (*HelmClient, error) {
	ret, err := NewClientFromRestConf(hClient.RestConfig, hClient.Namespace)
	if err != nil {
		return nil, err
	}
	ret.kubeClient = hClient.kubeClient
	return ret, nil
}

// mergeInstallOptions merges values of the provided chart to helm install options used by the client.
func mergeInstallOptions(chartSpec *hc.ChartSpec, installOptions *action.Install) {
	installOptions.CreateNamespace = chartSpec.CreateNamespace
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
	installOptions.DryRun = chartSpec.DryRun
	installOptions.SubNotes = chartSpec.SubNotes
}

// mergeUpgradeOptions merges values of the provided chart to helm upgrade options used by the client.
func mergeUpgradeOptions(chartSpec *hc.ChartSpec, upgradeOptions *action.Upgrade) {
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
	upgradeOptions.DryRun = chartSpec.DryRun
	upgradeOptions.SubNotes = chartSpec.SubNotes
}
