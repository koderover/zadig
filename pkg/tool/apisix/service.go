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

// Service represents an APISIX service configuration
type Service struct {
	ID              string                 `json:"id,omitempty"`
	Name            string                 `json:"name,omitempty"`
	Desc            string                 `json:"desc,omitempty"`
	Upstream        *Upstream              `json:"upstream,omitempty"`
	UpstreamID      string                 `json:"upstream_id,omitempty"`
	Plugins         map[string]interface{} `json:"plugins,omitempty"`
	Script          interface{}            `json:"script,omitempty"`
	Labels          map[string]string      `json:"labels,omitempty"`
	EnableWebsocket bool                   `json:"enable_websocket,omitempty"`
	Hosts           []string               `json:"hosts,omitempty"`
}

// ServiceResponse represents a single service response from APISIX Admin API
type ServiceResponse struct {
	Key           string   `json:"key"`
	Value         *Service `json:"value"`
	ModifiedIndex int64    `json:"modifiedIndex,omitempty"`
	CreatedIndex  int64    `json:"createdIndex,omitempty"`
}

// ServiceListResponse represents a list response from APISIX Admin API
type ServiceListResponse struct {
	List  []*ServiceResponse `json:"list"`
	Total int                `json:"total"`
}

// CreateService creates a new service with a random ID
// POST /apisix/admin/services
func (c *Client) CreateService(service *Service) (*ServiceResponse, error) {
	url := fmt.Sprintf("%s%s", c.Host, ServicesAPI)
	resp := new(ServiceResponse)

	err := c.Post(url, service, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %s", err)
	}

	return resp, nil
}

// UpdateService updates an existing service by ID
// PUT /apisix/admin/services/{id}
func (c *Client) UpdateService(id string, service *Service) (*ServiceResponse, error) {
	url := fmt.Sprintf("%s%s/%s", c.Host, ServicesAPI, id)
	resp := new(ServiceResponse)

	err := c.Put(url, service, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to update service: %s", err)
	}

	return resp, nil
}

// ListServices retrieves all services with optional pagination
// GET /apisix/admin/services
func (c *Client) ListServices(page, pageSize int) (*ServiceListResponse, error) {
	url := fmt.Sprintf("%s%s", c.Host, ServicesAPI)
	resp := new(ServiceListResponse)

	queries := make(map[string]string)
	if page > 0 {
		queries["page"] = fmt.Sprintf("%d", page)
	}
	if pageSize > 0 {
		queries["page_size"] = fmt.Sprintf("%d", pageSize)
	}

	err := c.Get(url, queries, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %s", err)
	}

	return resp, nil
}

// DeleteService deletes a service by ID
// DELETE /apisix/admin/services/{id}
func (c *Client) DeleteService(id string) error {
	url := fmt.Sprintf("%s%s/%s", c.Host, ServicesAPI, id)

	err := c.Delete(url, nil)
	if err != nil {
		return fmt.Errorf("failed to delete service: %s", err)
	}

	return nil
}

