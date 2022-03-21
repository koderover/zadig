/*
Copyright 2022 The KodeRover Authors.

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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	cm "github.com/chartmuseum/helm-push/pkg/chartmuseum"
	"github.com/chartmuseum/helm-push/pkg/helm"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/koderover/zadig/pkg/tool/log"
)

type ChartRepoClient struct {
	RepoURL string
	*cm.Client
}

func NewHelmChartRepoClient(url, userName, password string) (*ChartRepoClient, error) {
	client, err := cm.NewClient(
		cm.URL(url),
		cm.Username(userName),
		cm.Password(password),
		// TODO need support more auth types
	)
	if err != nil {
		return nil, err
	}
	return &ChartRepoClient{
		RepoURL: url,
		Client:  client,
	}, nil
}

// FetchIndexYaml fetch index.yaml from remote chart repo
func (client *ChartRepoClient) FetchIndexYaml() (*helm.Index, error) {
	resp, err := client.DownloadFile("index.yaml")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to download index.yaml")
	}

	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Wrapf(getChartmuseumError(b, resp.StatusCode), "failed to download index.yaml")
	}

	index, err := helm.LoadIndex(b)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse index.yaml")
	}
	return index, nil
}

func (client *ChartRepoClient) DownloadChart(chartName, chartVersion, basePath string) error {
	chartTGZName := fmt.Sprintf("%s-%s.tgz", chartName, chartVersion)
	chartTGZFilePath := filepath.Join(basePath, chartTGZName)

	entry, err := client.GetChartFromIndex(chartName, chartVersion)
	if err != nil {
		return errors.Wrapf(err, "failed to get chart [%s]-[%s] info", chartName, chartVersion)
	}
	if len(entry.URLs) == 0 {
		return errors.Wrapf(err, "failed to get chart [%s]-[%s] url", chartName, chartVersion)
	}

	response, err := client.downloadFileWithURL(entry.URLs[0])
	if err != nil {
		return errors.Wrapf(err, "failed to download file")
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download file failed with status code:%d", response.StatusCode)
	}

	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to read response data")
	}
	return ioutil.WriteFile(chartTGZFilePath, b, 0644)
}

// DownloadAndExpand downloads chart from repo and expand chart files from tarball
func (client *ChartRepoClient) DownloadAndExpand(chartName, chartVersion, localPath string) error {
	entry, err := client.GetChartFromIndex(chartName, chartVersion)
	if err != nil {
		return errors.Wrapf(err, "failed to get chart [%s]-[%s] info", chartName, chartVersion)
	}
	if len(entry.URLs) == 0 {
		return errors.Wrapf(err, "failed to get chart [%s]-[%s] url", chartName, chartVersion)
	}
	response, err := client.downloadFileWithURL(entry.URLs[0])
	if err != nil {
		return errors.Wrapf(err, "failed to download file")
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download file failed with status code:%d", response.StatusCode)
	}
	defer func() { _ = response.Body.Close() }()

	return chartutil.Expand(localPath, response.Body)
}

// PushChart push chart to remote repo
func (client *ChartRepoClient) PushChart(chartPackagePath string, force bool) error {
	log.Infof("pushing chart %s", filepath.Base(chartPackagePath))
	resp, err := client.UploadChartPackage(chartPackagePath, false)
	if err != nil {
		return errors.Wrapf(err, "failed to prepare pushing chart: %s", chartPackagePath)
	}

	defer resp.Body.Close()
	err = handlePushResponse(resp)
	if err != nil {
		return errors.Wrapf(err, "failed to push chart: %s ", chartPackagePath)
	}
	return nil
}

func getChartmuseumError(b []byte, code int) error {
	var er struct {
		Error string `json:"error"`
	}
	err := json.Unmarshal(b, &er)
	if err != nil || er.Error == "" {
		return errors.Errorf("%d: could not properly parse response JSON: %s", code, string(b))
	}
	return errors.Errorf("%d: %s", code, er.Error)
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

func (client *ChartRepoClient) GetChartFromIndex(chartName, chartVersion string) (*repo.ChartVersion, error) {
	index, err := client.FetchIndexYaml()
	if err != nil {
		return nil, errors.Wrapf(err, "fetch index failed")
	}
	for name, entries := range index.Entries {
		if strings.Compare(name, chartName) == 0 {
			for _, entry := range entries {
				if strings.Compare(entry.Version, chartVersion) == 0 {
					return entry, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("failed to find chart [%s]-[%s]", chartName, chartVersion)
}

// TODO this function should be deprecated
// we should use standard `helm pull` to provide better compatibility
func (client *ChartRepoClient) downloadFileWithURL(chartUrl string) (*http.Response, error) {
	u, _ := url.Parse(chartUrl)
	// chart url is absolute path in index.yaml, set url of option to full chart url and download directly
	// eg: chart repo hosted by gitee
	if u.Scheme == "http" || u.Scheme == "https" {
		client.Option(cm.URL(chartUrl))
		defer client.Option(cm.URL(client.RepoURL))
		return client.DownloadFile("")
	}
	// chart url in index.yaml is relative path, set url of option to the repoUrl and download by fileName
	// eg: chart repo hosted by harbor
	return client.DownloadFile(chartUrl)
}
