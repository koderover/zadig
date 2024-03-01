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

func (c *Client) ListOpenedProjectMergeRequests(owner, repo, targetBranch, key string, opts *ListOptions) ([]*gitlab.MergeRequest, error) {
	mergeRequests, err := wrap(paginated(func(o *gitlab.ListOptions) ([]interface{}, *gitlab.Response, error) {
		state := "opened"

		mopts := &gitlab.ListProjectMergeRequestsOptions{
			State:       &state,
			ListOptions: *o,
			Search:      &key,
		}
		if targetBranch != "" {
			mopts.TargetBranch = &targetBranch
		}
		mrs, r, err := c.MergeRequests.ListProjectMergeRequests(generateProjectName(owner, repo), mopts)
		var res []interface{}
		for _, mr := range mrs {
			res = append(res, mr)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*gitlab.MergeRequest
	mrs, ok := mergeRequests.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, mr := range mrs {
		res = append(res, mr.(*gitlab.MergeRequest))
	}

	return res, err
}

func (c *Client) ListChangedFiles(event *gitlab.MergeEvent) ([]string, error) {
	files := make([]string, 0)
	mergeRequest, err := wrap(c.MergeRequests.GetMergeRequestChanges(event.ObjectAttributes.TargetProjectID, event.ObjectAttributes.IID, nil))
	if err != nil || mergeRequest == nil {
		return nil, err
	}
	mr, ok := mergeRequest.(*gitlab.MergeRequest)
	if !ok {
		return nil, nil
	}
	for _, change := range mr.Changes {
		files = append(files, change.NewPath)
		files = append(files, change.OldPath)
	}

	return files, nil
}

//func (c *Client) CreateCommitDiscussion(owner, repo, commitHash, comment string) error {
//	args := &gitlab.CreateCommitDiscussionOptions{Body: &comment}
//	_, err := wrap(c.Discussions.CreateCommitDiscussion(generateProjectName(owner, repo), commitHash, args))
//	return err
//}
