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
	"path/filepath"

	cm "github.com/chartmuseum/helm-push/pkg/chartmuseum"
	"github.com/chartmuseum/helm-push/pkg/helm"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/koderover/zadig/pkg/tool/log"
)

type ChartRepoClient struct {
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
	return &ChartRepoClient{client}, nil
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

	response, err := client.DownloadFile(fmt.Sprintf("charts/%s", chartTGZName))
	if err != nil {
		return errors.Wrapf(err, "failed to download file")
	}

	if response.StatusCode != http.StatusOK {
		return errors.Wrapf(err, "download file failed")
	}
	defer response.Body.Close()

	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to read response data")
	}
	return ioutil.WriteFile(chartTGZFilePath, b, 0644)
}

// DownloadAndExpand downloads chart from repo and expand chart files from tarball
func (client *ChartRepoClient) DownloadAndExpand(chartName, chartVersion, localPath string) error {
	chartTGZName := fmt.Sprintf("%s-%s.tgz", chartName, chartVersion)
	response, err := client.DownloadFile(fmt.Sprintf("charts/%s", chartTGZName))
	if err != nil {
		return errors.Wrapf(err, "failed to download file")
	}

	if response.StatusCode != 200 {
		return errors.Wrapf(err, "download file failed")
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
