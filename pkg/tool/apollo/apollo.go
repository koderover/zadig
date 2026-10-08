/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package apollo

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	FormatYAML       = "yaml"
	FormatYML        = "yml"
	FormatJSON       = "json"
	FormatProperties = "properties"
	FormatXML        = "xml"
	FileItemKey      = "content"
	YAMLItemKey      = FileItemKey
)

func IsYAMLNamespace(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatYAML, FormatYML:
		return true
	default:
		return false
	}
}

func IsFileNamespace(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatYAML, FormatYML, FormatJSON, FormatXML:
		return true
	default:
		return false
	}
}

func NormalizeItemKey(format, key string) string {
	if IsFileNamespace(format) {
		return FileItemKey
	}
	return key
}

func NormalizeNamespaceName(name, format string) string {
	name = strings.TrimSpace(name)
	format = strings.ToLower(strings.TrimSpace(format))
	if !IsFileNamespace(format) || strings.EqualFold(filepath.Ext(name), "."+format) {
		return name
	}
	return name + "." + format
}

func ValidateNamespaceName(name string) error {
	name = strings.TrimSpace(name)
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if ext == "" {
		return nil
	}
	switch ext {
	case FormatYAML, FormatYML, FormatJSON, FormatProperties, FormatXML:
		return fmt.Errorf("namespace name must not include file suffix %q", ext)
	default:
		return nil
	}
}

type BriefNamespace struct {
	AppID         string `json:"appId"`
	Env           string `json:"env"`
	ClusterName   string `json:"clusterName"`
	NamespaceName string `json:"namespaceName"`
	Format        string `json:"format"`
}

type Namespace struct {
	AppID                      string   `json:"appId"`
	ClusterName                string   `json:"clusterName"`
	NamespaceName              string   `json:"namespaceName"`
	Comment                    string   `json:"comment"`
	Format                     string   `json:"format"`
	IsPublic                   bool     `json:"isPublic"`
	Items                      []*Items `json:"items"`
	DataChangeCreatedBy        string   `json:"dataChangeCreatedBy"`
	DataChangeLastModifiedBy   string   `json:"dataChangeLastModifiedBy"`
	DataChangeCreatedTime      string   `json:"dataChangeCreatedTime"`
	DataChangeLastModifiedTime string   `json:"dataChangeLastModifiedTime"`
}
type Items struct {
	Key                        string `json:"key"`
	Value                      string `json:"value"`
	Comment                    string `json:"comment"`
	DataChangeCreatedBy        string `json:"dataChangeCreatedBy"`
	DataChangeLastModifiedBy   string `json:"dataChangeLastModifiedBy"`
	DataChangeCreatedTime      string `json:"dataChangeCreatedTime"`
	DataChangeLastModifiedTime string `json:"dataChangeLastModifiedTime"`
}

type AppInfo struct {
	Name                       string `json:"name"`
	AppID                      string `json:"appId"`
	OrgID                      string `json:"orgId"`
	OrgName                    string `json:"orgName"`
	OwnerName                  string `json:"ownerName"`
	OwnerEmail                 string `json:"ownerEmail"`
	DataChangeCreatedBy        string `json:"dataChangeCreatedBy"`
	DataChangeLastModifiedBy   string `json:"dataChangeLastModifiedBy"`
	DataChangeCreatedTime      string `json:"dataChangeCreatedTime"`
	DataChangeLastModifiedTime string `json:"dataChangeLastModifiedTime"`
}

type EnvAndCluster struct {
	Env      string   `json:"env"`
	Clusters []string `json:"clusters"`
}

type CreateAppNamespaceArgs struct {
	Name                string `json:"name"`
	AppID               string `json:"appId"`
	Format              string `json:"format"`
	IsPublic            bool   `json:"isPublic"`
	Comment             string `json:"comment"`
	DataChangeCreatedBy string `json:"dataChangeCreatedBy"`
}

type AppNamespace struct {
	Name                     string `json:"name"`
	AppID                    string `json:"appId"`
	Format                   string `json:"format"`
	IsPublic                 bool   `json:"isPublic"`
	Comment                  string `json:"comment"`
	DataChangeCreatedBy      string `json:"dataChangeCreatedBy"`
	DataChangeLastModifiedBy string `json:"dataChangeLastModifiedBy"`
}

func (c *Client) ListApp() (list []*AppInfo, err error) {
	_, err = c.R().SetSuccessResult(&list).Get(c.BaseURL + "/openapi/v1/apps")
	return
}

func (c *Client) ListAppEnvsAndClusters(appID string) (envList []*EnvAndCluster, err error) {
	_, err = c.R().SetPathParams(map[string]string{
		"appId": appID,
	}).SetSuccessResult(&envList).Get(c.BaseURL + "/openapi/v1/apps/{appId}/envclusters")
	return
}

func (c *Client) CreateAppNamespace(appID string, args *CreateAppNamespaceArgs) (result *AppNamespace, err error) {
	_, err = c.R().SetPathParam("appId", appID).
		SetBodyJsonMarshal(args).
		SetSuccessResult(&result).
		Post(c.BaseURL + "/openapi/v1/apps/{appId}/appnamespaces")
	return
}

func (c *Client) ListAppNamespace(appID, env, cluster string) (list []*Namespace, err error) {
	_, err = c.R().SetPathParams(map[string]string{
		"env":         env,
		"appId":       appID,
		"clusterName": cluster,
	}).SetSuccessResult(&list).Get(c.BaseURL + "/openapi/v1/envs/{env}/apps/{appId}/clusters/{clusterName}/namespaces")
	return
}

func (c *Client) GetNamespace(appID, env, cluster, namespace string) (result *Namespace, err error) {
	_, err = c.R().SetPathParams(map[string]string{
		"env":           env,
		"appId":         appID,
		"clusterName":   cluster,
		"namespaceName": namespace,
	}).SetSuccessResult(&result).Get(c.BaseURL + "/openapi/v1/envs/{env}/apps/{appId}/clusters/{clusterName}/namespaces/{namespaceName}")
	return
}

func (c *Client) UpdateKeyVal(appID, env, cluster, namespace, key, val, updateUser string) error {
	type Req struct {
		Key       string `json:"key"`
		Value     string `json:"value"`
		ChangedBy string `json:"dataChangeLastModifiedBy"`
		CreatedBy string `json:"dataChangeCreatedBy"`
	}
	_, err := c.R().SetPathParams(map[string]string{
		"env":           env,
		"appId":         appID,
		"clusterName":   cluster,
		"namespaceName": namespace,
		"key":           key,
	}).SetBodyJsonMarshal(&Req{
		Key:       key,
		Value:     val,
		ChangedBy: updateUser,
		CreatedBy: updateUser,
	}).SetQueryParam("createIfNotExists", "true").
		Put(c.BaseURL + "/openapi/v1/envs/{env}/apps/{appId}/clusters/{clusterName}/namespaces/{namespaceName}/items/{key}")

	return err
}

type ReleaseArgs struct {
	ReleaseTitle   string `json:"releaseTitle"`
	ReleaseComment string `json:"releaseComment"`
	ReleasedBy     string `json:"releasedBy"`
}

func (c *Client) Release(appID, env, cluster, namespace string, args *ReleaseArgs) error {
	_, err := c.R().SetPathParams(map[string]string{
		"env":           env,
		"appId":         appID,
		"clusterName":   cluster,
		"namespaceName": namespace,
	}).SetBodyJsonMarshal(args).
		SetQueryParam("createIfNotExists", "true").
		Post(c.BaseURL + "/openapi/v1/envs/{env}/apps/{appId}/clusters/{clusterName}/namespaces/{namespaceName}/releases")
	return err
}
