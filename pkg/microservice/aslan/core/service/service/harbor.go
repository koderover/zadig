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

package service

import (
	"fmt"
	"net/url"
	"time"

	"go.uber.org/zap"
	"sigs.k8s.io/yaml"

	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type HarborProject struct {
	UpdateTime        string `json:"update_time"`
	OwnerName         string `json:"owner_name"`
	Name              string `json:"name"`
	Deleted           bool   `json:"deleted"`
	OwnerID           int    `json:"owner_id"`
	RepoCount         int    `json:"repo_count"`
	CreationTime      string `json:"creation_time"`
	Togglable         bool   `json:"togglable"`
	ProjectID         int    `json:"project_id"`
	CurrentUserRoleID int    `json:"current_user_role_id"`
	ChartCount        int    `json:"chart_count"`
	CveWhitelist      struct {
		Items []struct {
			CveID string `json:"cve_id"`
		} `json:"items"`
		ProjectID int `json:"project_id"`
		ID        int `json:"id"`
		ExpiresAt int `json:"expires_at"`
	} `json:"cve_whitelist"`
	Metadata struct {
		EnableContentTrust   string `json:"enable_content_trust"`
		AutoScan             string `json:"auto_scan"`
		Severity             string `json:"severity"`
		ReuseSysCveWhitelist string `json:"reuse_sys_cve_whitelist"`
		Public               string `json:"public"`
		PreventVul           string `json:"prevent_vul"`
	} `json:"metadata"`
}

type HarborChartRepo struct {
	Updated       string `json:"updated"`
	Name          string `json:"name"`
	Created       string `json:"created"`
	Deprecated    bool   `json:"deprecated"`
	TotalVersions int    `json:"total_versions"`
	LatestVersion string `json:"latest_version"`
	Home          string `json:"home"`
	Icon          string `json:"icon"`
}

type HarborChartDetail struct {
	Metadata struct {
		Name        string    `json:"name"`
		Version     string    `json:"version"`
		Description string    `json:"description"`
		APIVersion  string    `json:"apiVersion"`
		AppVersion  string    `json:"appVersion"`
		Urls        []string  `json:"urls"`
		Created     time.Time `json:"created"`
		Digest      string    `json:"digest"`
	} `json:"metadata"`
	Dependencies []string               `json:"dependencies"`
	YamlValues   map[string]interface{} `json:"yaml_values"`
	Files        struct {
		ValuesYaml string `json:"values.yaml"`
	} `json:"files"`
	Security struct {
		Signature struct {
			Signed   bool   `json:"signed"`
			ProvFile string `json:"prov_file"`
		} `json:"signature"`
	} `json:"security"`
	Labels []string `json:"labels"`
}

type HarborChartVersion struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	APIVersion  string    `json:"apiVersion"`
	AppVersion  string    `json:"appVersion"`
	Urls        []string  `json:"urls"`
	Created     time.Time `json:"created"`
	Digest      string    `json:"digest"`
	Labels      []string  `json:"labels"`
}

func GetHarborURL(path string) (string, error) {
	if helmIntegrations, err := commonrepo.NewHelmRepoColl().List(); err == nil {
		if len(helmIntegrations) > 0 {
			helmIntegration := helmIntegrations[0]
			uri, err := url.Parse(helmIntegration.URL)
			if err != nil {
				return "", fmt.Errorf("url prase failed")
			}
			return fmt.Sprintf("%s://%s:%s@%s/%s", uri.Scheme, helmIntegration.Username, helmIntegration.Password, uri.Host, path), nil
		}
	}
	return "", fmt.Errorf("harbor integration not found")
}

func ListHarborProjects(page, pageSize int, log *zap.SugaredLogger) ([]*HarborProject, error) {
	harborProjects := make([]*HarborProject, 0)

	url, err := GetHarborURL(fmt.Sprintf("api/projects?page=%d&page_size=%d", page, pageSize))
	if err != nil { //前端根据空数组判断显示对应文案,后端不再做错误信息返回
		return harborProjects, nil
	}

	_, err = httpclient.Get(url, httpclient.SetResult(&harborProjects))
	if err != nil {
		return harborProjects, fmt.Errorf("harbor request url:%s err:%v", url, err)
	}

	return harborProjects, nil
}

func ListHarborChartRepos(project string, log *zap.SugaredLogger) ([]*HarborChartRepo, error) {
	harborChartRepos := make([]*HarborChartRepo, 0)

	url, err := GetHarborURL(fmt.Sprintf("api/chartrepo/%s/charts", project))
	if err != nil { //前端根据空数组判断显示对应文案,后端不再做错误信息返回
		return harborChartRepos, nil
	}

	_, err = httpclient.Get(url, httpclient.SetResult(&harborChartRepos))
	if err != nil {
		return harborChartRepos, fmt.Errorf("harbor request url:%s err:%v", url, err)
	}

	return harborChartRepos, nil
}

func ListHarborChartVersions(project, chartName string, log *zap.SugaredLogger) ([]*HarborChartVersion, error) {
	harborChartVersions := make([]*HarborChartVersion, 0)

	url, err := GetHarborURL(fmt.Sprintf("api/chartrepo/%s/charts/%s", project, chartName))
	if err != nil { //前端根据空数组判断显示对应文案,后端不再做错误信息返回
		return harborChartVersions, nil
	}

	_, err = httpclient.Get(url, httpclient.SetResult(&harborChartVersions))
	if err != nil {
		return harborChartVersions, fmt.Errorf("harbor request url:%s err:%v", url, err)
	}

	return harborChartVersions, nil
}

func FindHarborChartDetail(project, chartName, version string, log *zap.SugaredLogger) (*HarborChartDetail, error) {
	harborChartDetail := &HarborChartDetail{}
	url, err := GetHarborURL(fmt.Sprintf("api/chartrepo/%s/charts/%s/%s", project, chartName, version))
	if err != nil {
		return harborChartDetail, fmt.Errorf("GetHarborUrl err:%v", err)
	}

	_, err = httpclient.Get(url, httpclient.SetResult(harborChartDetail))
	if err != nil {
		return harborChartDetail, fmt.Errorf("harbor request url:%s err:%v", url, err)
	}

	if harborChartDetail != nil {
		yamlValuesByte, err := yaml.YAMLToJSON([]byte(harborChartDetail.Files.ValuesYaml))
		if err != nil {
			log.Errorf("harbor YAMLToJSON err:%v", err)
			return harborChartDetail, nil
		}
		var valuesMap map[string]interface{}
		if err := yaml.Unmarshal(yamlValuesByte, &valuesMap); err != nil {
			log.Errorf("harbor Unmarshal err:%v", err)
			return harborChartDetail, nil
		}
		harborChartDetail.YamlValues = valuesMap
	}

	return harborChartDetail, nil
}
