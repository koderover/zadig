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
	"fmt"
)

// Route represents an APISIX route configuration
type Route struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Desc        string                 `json:"desc,omitempty"`
	URI         string                 `json:"uri,omitempty"`
	URIs        []string               `json:"uris,omitempty"`
	Host        string                 `json:"host,omitempty"`
	Hosts       []string               `json:"hosts,omitempty"`
	RemoteAddr  string                 `json:"remote_addr,omitempty"`
	RemoteAddrs []string               `json:"remote_addrs,omitempty"`
	Methods     []string               `json:"methods,omitempty"`
	Priority    int                    `json:"priority,omitempty"`
	Vars        [][]interface{}        `json:"vars,omitempty"`
	FilterFunc  string                 `json:"filter_func,omitempty"`
	Plugins     map[string]interface{} `json:"plugins,omitempty"`
	Script      interface{}            `json:"script,omitempty"`
	Upstream    *Upstream              `json:"upstream,omitempty"`
	UpstreamID  string                 `json:"upstream_id,omitempty"`
	ServiceID   string                 `json:"service_id,omitempty"`
	PluginConfigID string              `json:"plugin_config_id,omitempty"`
	Labels      map[string]string      `json:"labels,omitempty"`
	Timeout     *UpstreamTimeout       `json:"timeout,omitempty"`
	EnableWebsocket bool               `json:"enable_websocket,omitempty"`
	Status      int                    `json:"status,omitempty"` // 1: enable, 0: disable
}

// RouteResponse represents a single route response from APISIX Admin API
type RouteResponse struct {
	Key           string `json:"key"`
	Value         *Route `json:"value"`
	ModifiedIndex int64  `json:"modifiedIndex,omitempty"`
	CreatedIndex  int64  `json:"createdIndex,omitempty"`
}

// RouteListResponse represents a list response from APISIX Admin API
type RouteListResponse struct {
	List  []*RouteResponse `json:"list"`
	Total int              `json:"total"`
}

// CreateRoute creates a new route with a random ID
// POST /apisix/admin/routes
func (c *Client) CreateRoute(route *Route) (*RouteResponse, error) {
	url := fmt.Sprintf("%s%s", c.Host, RoutesAPI)
	resp := new(RouteResponse)

	err := c.Post(url, route, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to create route: %s", err)
	}

	return resp, nil
}

// UpdateRoute updates an existing route by ID
// PUT /apisix/admin/routes/{id}
func (c *Client) UpdateRoute(id string, route *Route) (*RouteResponse, error) {
	url := fmt.Sprintf("%s%s/%s", c.Host, RoutesAPI, id)
	resp := new(RouteResponse)

	err := c.Put(url, route, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to update route: %s", err)
	}

	return resp, nil
}

// ListRoutes retrieves all routes with optional pagination
// GET /apisix/admin/routes
func (c *Client) ListRoutes(page, pageSize int) (*RouteListResponse, error) {
	url := fmt.Sprintf("%s%s", c.Host, RoutesAPI)
	resp := new(RouteListResponse)

	queries := make(map[string]string)
	if page > 0 {
		queries["page"] = fmt.Sprintf("%d", page)
	}
	if pageSize > 0 {
		queries["page_size"] = fmt.Sprintf("%d", pageSize)
	}

	err := c.Get(url, queries, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes: %s", err)
	}

	return resp, nil
}

// DeleteRoute deletes a route by ID
// DELETE /apisix/admin/routes/{id}
func (c *Client) DeleteRoute(id string) error {
	url := fmt.Sprintf("%s%s/%s", c.Host, RoutesAPI, id)

	err := c.Delete(url, nil)
	if err != nil {
		return fmt.Errorf("failed to delete route: %s", err)
	}

	return nil
}

