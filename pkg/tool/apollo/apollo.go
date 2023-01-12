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
	"net/http"

	"github.com/pkg/errors"
)

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

func (c *Client) ListNamespace(appID, env, cluster string) (list []*Namespace, err error) {
	resp, err := c.R().SetPathParams(map[string]string{
		"env":         env,
		"appId":       appID,
		"clusterName": cluster,
	}).Get(c.BaseURL + "/openapi/v1/envs/{env}/apps/{appId}/clusters/{clusterName}/namespaces")
	if err != nil {
		return nil, errors.Wrap(err, "send request")
	}
	if resp.GetStatusCode() != http.StatusOK {
		return nil, errors.Errorf("unexpected status code %d, body: %s", resp.GetStatusCode(), resp.String())
	}
	if err = resp.UnmarshalJson(&list); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}
	return
}

func (c *Client) GetNamespace(appID, env, cluster, namespace string) (*Namespace, error) {
	resp, err := c.R().SetPathParams(map[string]string{
		"env":           env,
		"appId":         appID,
		"clusterName":   cluster,
		"namespaceName": namespace,
	}).Get(c.BaseURL + "/openapi/v1/envs/{env}/apps/{appId}/clusters/{clusterName}/namespaces/{namespaceName}")
	if err != nil {
		return nil, errors.Wrap(err, "send request")
	}
	if resp.GetStatusCode() != http.StatusOK {
		return nil, errors.Errorf("unexpected status code %d, body: %s", resp.GetStatusCode(), resp.String())
	}
	result := new(Namespace)
	if err = resp.UnmarshalJson(result); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}
	return result, nil
}

func (c *Client) UpdateKeyVal(appID, env, cluster, namespace, key, val, updateUser string) error {
	type Req struct {
		Key       string `json:"key"`
		Value     string `json:"value"`
		ChangedBy string `json:"dataChangeLastModifiedBy"`
		CreatedBy string `json:"dataChangeCreatedBy"`
	}
	resp, err := c.R().SetPathParams(map[string]string{
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

	if err != nil {
		return errors.Wrap(err, "send request")
	}
	if resp.GetStatusCode() != http.StatusOK {
		return errors.Errorf("unexpected status code %d, body: %s", resp.GetStatusCode(), resp.String())
	}
	return nil
}

type ReleaseArgs struct {
	ReleaseTitle   string `json:"releaseTitle"`
	ReleaseComment string `json:"releaseComment"`
	ReleasedBy     string `json:"releasedBy"`
}

func (c *Client) Release(appID, env, cluster, namespace string, args *ReleaseArgs) error {
	resp, err := c.R().SetPathParams(map[string]string{
		"env":           env,
		"appId":         appID,
		"clusterName":   cluster,
		"namespaceName": namespace,
	}).SetBodyJsonMarshal(args).
		SetQueryParam("createIfNotExists", "true").
		Post(c.BaseURL + "/openapi/v1/envs/{env}/apps/{appId}/clusters/{clusterName}/namespaces/{namespaceName}/releases")
	if err != nil {
		return errors.Wrap(err, "send request")
	}
	if resp.GetStatusCode() != http.StatusOK {
		return errors.Errorf("unexpected status code %d, body: %s", resp.GetStatusCode(), resp.String())
	}
	return nil
}
