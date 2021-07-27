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

package git

import (
	"github.com/google/go-github/v35/github"
	"github.com/xanzy/go-gitlab"
)

type RepositoryCommit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

func ToRepositoryCommit(obj interface{}) *RepositoryCommit {
	switch o := obj.(type) {
	case *github.RepositoryCommit:
		return &RepositoryCommit{SHA: o.GetSHA(), Message: o.GetCommit().GetMessage()}
	case *gitlab.Commit:
		return &RepositoryCommit{SHA: o.ID, Message: o.Message}
	default:
		return nil
	}
}
