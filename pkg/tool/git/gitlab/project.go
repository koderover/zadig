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

	"github.com/koderover/zadig/pkg/tool/git"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/util/boolptr"
)

func (c *Client) ListUserProjects(owner, keyword string, opts *ListOptions) ([]*gitlab.Project, error) {
	projects, err := wrap(paginated(func(o *gitlab.ListOptions) ([]interface{}, *gitlab.Response, error) {
		encodeOwner := strings.Replace(owner, ".", "%2e", -1)
		popts := &gitlab.ListProjectsOptions{
			ListOptions: *o,
		}

		if keyword != "" {
			popts.Search = &keyword
		}
		ps, r, err := c.Projects.ListUserProjects(encodeOwner, popts)
		var res []interface{}
		for _, p := range ps {
			res = append(res, p)
		}
		return res, r, err
	}, opts))

	var res []*gitlab.Project
	ps, ok := projects.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, p := range ps {
		res = append(res, p.(*gitlab.Project))
	}

	return res, err
}

func (c *Client) AddProjectHook(owner, repo string, hook *git.Hook) (*gitlab.ProjectHook, error) {
	opts := &gitlab.AddProjectHookOptions{
		URL:   &hook.URL,
		Token: &hook.Secret,
	}
	opts = addEventsToProjectHookOptions(hook.Events, opts)
	created, err := wrap(c.Projects.AddProjectHook(generateProjectName(owner, repo), opts))
	if err != nil {
		return nil, err
	}

	if h, ok := created.(*gitlab.ProjectHook); ok {
		return h, nil
	}

	return nil, err
}

func (c *Client) DeleteProjectHook(owner, repo string, id int) error {
	err := wrapError(c.Projects.DeleteProjectHook(generateProjectName(owner, repo), id))
	if httpclient.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) ListProjectHooks(owner, repo string, opts *ListOptions) ([]*gitlab.ProjectHook, error) {
	hooks, err := wrap(paginated(func(o *gitlab.ListOptions) ([]interface{}, *gitlab.Response, error) {
		hs, r, err := c.Projects.ListProjectHooks(generateProjectName(owner, repo), (*gitlab.ListProjectHooksOptions)(o))
		var res []interface{}
		for _, h := range hs {
			res = append(res, h)
		}
		return res, r, err
	}, opts))

	var res []*gitlab.ProjectHook
	hs, ok := hooks.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, hook := range hs {
		res = append(res, hook.(*gitlab.ProjectHook))
	}

	return res, err
}

func (c *Client) GetProjectID(owner, repo string) (int, error) {
	p, err := c.getProject(owner, repo)
	if err != nil {
		return 0, err
	}

	return p.ID, nil
}

func (c *Client) getProject(owner, repo string) (*gitlab.Project, error) {
	project, err := wrap(c.Projects.GetProject(generateProjectName(owner, repo), nil))
	if err != nil {
		return nil, err
	}

	if p, ok := project.(*gitlab.Project); ok {
		return p, nil
	}

	return nil, err
}

func addEventsToProjectHookOptions(events []string, opts *gitlab.AddProjectHookOptions) *gitlab.AddProjectHookOptions {
	for _, evt := range events {
		switch evt {
		case git.PushEvent:
			opts.PushEvents = boolptr.True()
		case git.PullRequestEvent:
			opts.MergeRequestsEvents = boolptr.True()
		case git.BranchOrTagCreateEvent:
			opts.TagPushEvents = boolptr.True()
		}
	}

	return opts
}
