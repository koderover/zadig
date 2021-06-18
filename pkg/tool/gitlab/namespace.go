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

package gitlab

import (
	"strings"

	"github.com/xanzy/go-gitlab"
)

type Namespace struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Kind string `json:"kind"`
}

func (c *Client) ListNamespaces(keyword string) ([]*Namespace, error) {
	opts := &gitlab.ListNamespacesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	// gitlab search works only when character length > 2
	if keyword != "" && len(keyword) > 2 {
		opts.Search = &keyword
	}

	var respNs []*Namespace
	namespaces, _, err := c.Namespaces.ListNamespaces(opts)
	if err != nil {
		return nil, err
	}

	for _, ns := range namespaces {
		respN := &Namespace{
			Name: ns.Path,
			Path: ns.FullPath,
			Kind: ns.Kind,
		}

		if len(keyword) > 0 && !strings.Contains(strings.ToLower(ns.Path), strings.ToLower(keyword)) {
			// filter
		} else {
			respNs = append(respNs, respN)
		}
	}

	return respNs, nil
}
