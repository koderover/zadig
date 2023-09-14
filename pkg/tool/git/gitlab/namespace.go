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
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) listUsers(keyword string, opts *ListOptions) ([]*gitlab.Namespace, error) {
	namespaces, err := wrap(paginated(func(o *gitlab.ListOptions) ([]interface{}, *gitlab.Response, error) {
		mopts := &gitlab.ListNamespacesOptions{
			ListOptions: *o,
		}
		// gitlab search works only when character length > 2
		if keyword != "" && len(keyword) > 2 {
			mopts.Search = &keyword
		}
		ns, r, err := c.Namespaces.ListNamespaces(mopts)
		var res []interface{}
		for _, n := range ns {
			res = append(res, n)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*gitlab.Namespace
	ns, ok := namespaces.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, n := range ns {
		namespace := n.(*gitlab.Namespace)
		log.Infof("--------- namespace kind is %v", namespace.Kind)
		if namespace.Kind == "user" {
			res = append(res)
		}
	}

	return res, err
}

func (c *Client) listGroups(keyword string, opts *ListOptions) ([]*gitlab.Namespace, error) {
	listFunc := func(o *gitlab.ListOptions) ([]interface{}, *gitlab.Response, error) {
		opts := &gitlab.ListGroupsOptions{
			ListOptions: *o,
		}
		// gitlab search works only when character length > 2
		if keyword != "" && len(keyword) > 2 {
			opts.Search = &keyword
		}
		ns, r, err := c.Groups.ListGroups(opts)
		var res []interface{}
		for _, n := range ns {
			res = append(res, n)
		}
		return res, r, err
	}
	groups, err := wrap(paginated(listFunc, opts))

	if err != nil {
		return nil, err
	}

	var res []*gitlab.Namespace
	ns, ok := groups.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, n := range ns {
		groupInfo := n.(*gitlab.Group)
		res = append(res, &gitlab.Namespace{
			Name:     groupInfo.Name,
			Path:     groupInfo.Path,
			FullPath: groupInfo.FullPath,
			Kind:     "group",
		})
	}
	return res, err
}

// Deprecated
// listNamespaces only returns groups owned by the user
func (c *Client) listNamespaces(keyword string, opts *ListOptions) ([]*gitlab.Namespace, error) {
	namespaces, err := wrap(paginated(func(o *gitlab.ListOptions) ([]interface{}, *gitlab.Response, error) {
		mopts := &gitlab.ListNamespacesOptions{
			ListOptions: *o,
		}
		// gitlab search works only when character length > 2
		if keyword != "" && len(keyword) > 2 {
			mopts.Search = &keyword
		}
		ns, r, err := c.Namespaces.ListNamespaces(mopts)
		var res []interface{}
		for _, n := range ns {
			res = append(res, n)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*gitlab.Namespace
	ns, ok := namespaces.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, n := range ns {
		res = append(res, n.(*gitlab.Namespace))
	}

	return res, err
}

func (c *Client) ListNamespaces(keyword string, opts *ListOptions) ([]*gitlab.Namespace, error) {
	ret := make([]*gitlab.Namespace, 0)
	users, err := c.listUsers(keyword, opts)
	if err != nil {
		return nil, err
	}
	ret = append(ret, users...)

	groups, err := c.listGroups(keyword, opts)
	if err != nil {
		return nil, err
	}
	ret = append(ret, groups...)
	return ret, nil
}
