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

// Proto represents an APISIX proto configuration for gRPC
type Proto struct {
	ID      string            `json:"id,omitempty"`
	Name    string            `json:"name,omitempty"`
	Desc    string            `json:"desc,omitempty"`
	Content string            `json:"content,omitempty"` // Content of .proto or .pb files
	Labels  map[string]string `json:"labels,omitempty"`
}

// ProtoResponse represents a single proto response from APISIX Admin API
type ProtoResponse struct {
	Key           string `json:"key"`
	Value         *Proto `json:"value"`
	ModifiedIndex int64  `json:"modifiedIndex,omitempty"`
	CreatedIndex  int64  `json:"createdIndex,omitempty"`
}

// ProtoListResponse represents a list response from APISIX Admin API
type ProtoListResponse struct {
	List  []*ProtoResponse `json:"list"`
	Total int              `json:"total"`
}

// ProtosAPI is the endpoint for proto resources
const ProtosAPI = "/apisix/admin/protos"

// CreateProto creates a new proto with a random ID
// POST /apisix/admin/protos
func (c *Client) CreateProto(proto *Proto) (*ProtoResponse, error) {
	url := fmt.Sprintf("%s%s", c.Host, ProtosAPI)
	resp := new(ProtoResponse)

	err := c.Post(url, proto, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to create proto: %s", err)
	}

	return resp, nil
}

// UpdateProto updates an existing proto by ID
// PUT /apisix/admin/protos/{id}
func (c *Client) UpdateProto(id string, proto *Proto) (*ProtoResponse, error) {
	url := fmt.Sprintf("%s%s/%s", c.Host, ProtosAPI, id)
	resp := new(ProtoResponse)

	err := c.Put(url, proto, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to update proto: %s", err)
	}

	return resp, nil
}

// ListProtos retrieves all protos with optional pagination
// GET /apisix/admin/protos
func (c *Client) ListProtos(page, pageSize int) (*ProtoListResponse, error) {
	url := fmt.Sprintf("%s%s", c.Host, ProtosAPI)
	resp := new(ProtoListResponse)

	queries := make(map[string]string)
	if page > 0 {
		queries["page"] = fmt.Sprintf("%d", page)
	}
	if pageSize > 0 {
		queries["page_size"] = fmt.Sprintf("%d", pageSize)
	}

	err := c.Get(url, queries, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to list protos: %s", err)
	}

	return resp, nil
}

// DeleteProto deletes a proto by ID
// DELETE /apisix/admin/protos/{id}
func (c *Client) DeleteProto(id string) error {
	url := fmt.Sprintf("%s%s/%s", c.Host, ProtosAPI, id)

	err := c.Delete(url, nil)
	if err != nil {
		return fmt.Errorf("failed to delete proto: %s", err)
	}

	return nil
}

