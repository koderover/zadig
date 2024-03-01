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

import "github.com/xanzy/go-gitlab"

// ListTags lists branches by projectID <- urlEncode(namespace/projectName)
func (c *Client) ListTags(owner, repo string, opts *ListOptions, key string) ([]*gitlab.Tag, error) {
	tags, err := wrap(paginated(func(o *gitlab.ListOptions) ([]interface{}, *gitlab.Response, error) {
		ts, r, err := c.Tags.ListTags(generateProjectName(owner, repo), &gitlab.ListTagsOptions{ListOptions: *o, Search: &key})
		var res []interface{}
		for _, t := range ts {
			res = append(res, t)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*gitlab.Tag
	ts, ok := tags.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, t := range ts {
		res = append(res, t.(*gitlab.Tag))
	}

	return res, err
}
