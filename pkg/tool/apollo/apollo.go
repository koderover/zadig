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
	}).SetSuccessResult(result).Get(c.BaseURL + "/openapi/v1/envs/{env}/apps/{appId}/clusters/{clusterName}/namespaces/{namespaceName}")
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
