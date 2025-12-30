/*
Copyright 2025 The KodeRover Authors.

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

package apisix

import (
	"encoding/json"
	"fmt"
)

// Upstream represents an APISIX upstream configuration
type Upstream struct {
	ID            string                 `json:"id,omitempty"`
	Name          string                 `json:"name,omitempty"`
	Desc          string                 `json:"desc,omitempty"`
	Type          string                 `json:"type,omitempty"`           // roundrobin, chash, ewma, least_conn
	Nodes         interface{}            `json:"nodes,omitempty"`          // Can be map[string]int or []UpstreamNode
	ServiceName   string                 `json:"service_name,omitempty"`   // for service discovery
	DiscoveryType string                 `json:"discovery_type,omitempty"` // dns, consul, nacos, etc.
	HashOn        string                 `json:"hash_on,omitempty"`        // vars, header, cookie, consumer, vars_combinations
	Key           string                 `json:"key,omitempty"`            // hash key
	Checks        map[string]interface{} `json:"checks,omitempty"`         // health check configuration
	Retries       int                    `json:"retries,omitempty"`
	RetryTimeout  int                    `json:"retry_timeout,omitempty"`
	Timeout       *UpstreamTimeout       `json:"timeout,omitempty"`
	Scheme        string                 `json:"scheme,omitempty"` // http, https, grpc, grpcs
	Labels        map[string]string      `json:"labels,omitempty"`
	PassHost      string                 `json:"pass_host,omitempty"`      // pass, node, rewrite
	UpstreamHost  string                 `json:"upstream_host,omitempty"`  // used when pass_host is rewrite
	KeepalivePool *KeepalivePool         `json:"keepalive_pool,omitempty"`
	TLS           *UpstreamTLS           `json:"tls,omitempty"`
}

// UpstreamNode represents a node in array format
type UpstreamNode struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Weight   int    `json:"weight"`
	Priority int    `json:"priority,omitempty"`
}

// UpstreamTimeout defines timeout settings for upstream
type UpstreamTimeout struct {
	Connect int `json:"connect,omitempty"`
	Send    int `json:"send,omitempty"`
	Read    int `json:"read,omitempty"`
}

// KeepalivePool defines keepalive pool settings
type KeepalivePool struct {
	Size        int `json:"size,omitempty"`
	IdleTimeout int `json:"idle_timeout,omitempty"`
	Requests    int `json:"requests,omitempty"`
}

// UpstreamTLS defines TLS settings for upstream
type UpstreamTLS struct {
	ClientCert string `json:"client_cert,omitempty"`
	ClientKey  string `json:"client_key,omitempty"`
	Verify     bool   `json:"verify,omitempty"`
}

// UpstreamResponse represents a single upstream response from APISIX Admin API
type UpstreamResponse struct {
	Key           string    `json:"key"`
	Value         *Upstream `json:"value"`
	ModifiedIndex int64     `json:"modifiedIndex,omitempty"`
	CreatedIndex  int64     `json:"createdIndex,omitempty"`
}

// UpstreamListResponse represents a list response from APISIX Admin API
type UpstreamListResponse struct {
	List  []*UpstreamResponse `json:"list"`
	Total int                 `json:"total"`
}

// CreateUpstream creates a new upstream with a random ID
// POST /apisix/admin/upstreams
func (c *Client) CreateUpstream(upstream *Upstream) (*UpstreamResponse, error) {
	url := fmt.Sprintf("%s%s", c.Host, UpstreamsAPI)
	resp := new(UpstreamResponse)

	err := c.Post(url, upstream, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream: %s", err)
	}

	return resp, nil
}

// UpdateUpstream updates an existing upstream by ID
// PUT /apisix/admin/upstreams/{id}
func (c *Client) UpdateUpstream(id string, upstream *Upstream) (*UpstreamResponse, error) {
	url := fmt.Sprintf("%s%s/%s", c.Host, UpstreamsAPI, id)
	resp := new(UpstreamResponse)

	err := c.Put(url, upstream, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to update upstream: %s", err)
	}

	return resp, nil
}

// ListUpstreams retrieves all upstreams with optional pagination
// GET /apisix/admin/upstreams
// Supports pagination with page and page_size parameters
func (c *Client) ListUpstreams(page, pageSize int) (*UpstreamListResponse, error) {
	url := fmt.Sprintf("%s%s", c.Host, UpstreamsAPI)
	resp := new(UpstreamListResponse)

	queries := make(map[string]string)
	if page > 0 {
		queries["page"] = fmt.Sprintf("%d", page)
	}
	if pageSize > 0 {
		queries["page_size"] = fmt.Sprintf("%d", pageSize)
	}

	err := c.Get(url, queries, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to list upstreams: %s", err)
	}

	return resp, nil
}

// DeleteUpstream deletes an upstream by ID
// DELETE /apisix/admin/upstreams/{id}
func (c *Client) DeleteUpstream(id string) error {
	url := fmt.Sprintf("%s%s/%s", c.Host, UpstreamsAPI, id)

	resp := make(map[string]interface{})

	err := c.Delete(url, &resp)
	if err != nil {
		return fmt.Errorf("failed to delete upstream: %s", err)
	}

	deleteResponse, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal delete response: %s", err)
	}
	fmt.Println(string(deleteResponse))

	return nil
}

