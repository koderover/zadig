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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	hc "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	"helm.sh/helm/v3/pkg/strvals"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/tool/log"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
)

type HelmClient struct {
	*hc.HelmClient
}

// NewClientFromRestConf returns a new Helm client constructed with the provided REST config options
func NewClientFromRestConf(restConfig *rest.Config, ns string) (hc.Client, error) {
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
	}, nil
}

type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// MergeOverrideValues merge override yaml and override kvs
// defaultValues overrideYaml used for -f option
// overrideValues used for --set option
func MergeOverrideValues(valuesYaml, defaultValues, overrideYaml, overrideValues string) (string, error) {

	// merge files for helm -f option
	// precedence from low to high: valuesYaml defaultValues overrideYaml
	valuesMap, err := yamlutil.MergeAndUnmarshal([][]byte{[]byte(valuesYaml), []byte(defaultValues), []byte(overrideYaml)})
	if err != nil {
		return "", err
	}

	// merge kv values for helm --set option
	if overrideValues != "" {
		kvList := make([]*KV, 0)
		err = json.Unmarshal([]byte(overrideValues), &kvList)
		if err != nil {
			return "", err
		}
		kvStr := make([]string, 0)
		for _, kv := range kvList {
			kvStr = append(kvStr, fmt.Sprintf("%s=%s", kv.Key, kv.Value))
		}
		// override values for --set option
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
	// find history of particular release
	releases, err := hClient.ListReleaseHistory(spec.ReleaseName, 10)
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

	// release with deployed/failed/superseded status: normal upgrade operation
	if lastRelease.Info.Status == release.StatusDeployed || lastRelease.Info.Status == release.StatusFailed || lastRelease.Info.Status == release.StatusSuperseded {
		return false, nil
	}

	// find deployed revision with status deployed from history, would be upgrade operation
	for _, rel := range releases {
		if rel.Info.Status == release.StatusDeployed {
			return false, nil
		}
	}

	// if replace set to true, install will be a legal operation
	if st := lastRelease.Info.Status; spec.Replace && (st == release.StatusUninstalled || st == release.StatusFailed) {
		return true, nil
	}

	return false, fmt.Errorf("can't install or upgrade chart with status: %s", lastRelease.Info.Status)
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
	client := action.NewInstall(c.ActionConfig)
	mergeInstallOptions(spec, client)

	if client.Version == "" {
		client.Version = ">0.0.0-0"
	}

	helmChart, chartPath, err := hClient.getChart(spec.ChartName, &client.ChartPathOptions)
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
			if !client.DependencyUpdate {
				return nil, err
			}
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
		}
	}

	values, err := spec.GetValuesMap()
	if err != nil {
		return nil, err
	}

	rel, err := client.RunWithContext(ctx, helmChart, values)
	if err != nil {
		return rel, err
	}

	c.DebugLog("release installed successfully: %s/%s-%s", rel.Name, rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)

	return rel, nil
}

func (hClient *HelmClient) upgradeChart(ctx context.Context, spec *hc.ChartSpec) (*release.Release, error) {
	c := hClient.HelmClient
	client := action.NewUpgrade(c.ActionConfig)
	mergeUpgradeOptions(spec, client)

	if client.Version == "" {
		client.Version = ">0.0.0-0"
	}

	helmChart, _, err := hClient.getChart(spec.ChartName, &client.ChartPathOptions)
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

	rel, err := client.RunWithContext(ctx, spec.ReleaseName, helmChart, values)
	if err != nil {
		return rel, err
	}

	c.DebugLog("release upgraded successfully: %s/%s-%s", rel.Name, rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)

	return rel, nil
}

// InstallOrUpgradeChart install or upgrade helm chart, use the same rule with helm to determine weather to install or upgrade
func (hClient *HelmClient) InstallOrUpgradeChart(ctx context.Context, spec *hc.ChartSpec) (*release.Release, error) {
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
