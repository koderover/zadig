/*
Copyright 2026 The KodeRover Authors.

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

package job

import (
	"testing"

	"github.com/koderover/zadig/v2/pkg/types"
)

func TestApplyReposClearsDisabledCommit(t *testing.T) {
	base := []*types.Repository{{
		RepoOwner:    "owner",
		RepoName:     "repo",
		Branch:       "main",
		EnableCommit: true,
		CommitID:     "old-sha",
	}}
	input := []*types.Repository{{
		RepoOwner:    "owner",
		RepoName:     "repo",
		Branch:       "main",
		EnableCommit: false,
		CommitID:     "old-sha",
	}}

	got := applyRepos(base, input)

	if got[0].EnableCommit {
		t.Fatalf("expected commit mode to be disabled")
	}
	if got[0].CommitID != "" {
		t.Fatalf("expected commit ID to be cleared, got %q", got[0].CommitID)
	}
	if base[0].EnableCommit != true || base[0].CommitID != "old-sha" {
		t.Fatalf("applyRepos mutated the base repository: %+v", base[0])
	}
	if input[0].EnableCommit != false || input[0].CommitID != "old-sha" {
		t.Fatalf("applyRepos mutated the input repository: %+v", input[0])
	}
}

func TestApplyReposPreservesEnabledCommit(t *testing.T) {
	base := []*types.Repository{{
		RepoOwner:    "owner",
		RepoName:     "repo",
		Branch:       "main",
		EnableCommit: false,
	}}
	input := []*types.Repository{{
		RepoOwner:    "owner",
		RepoName:     "repo",
		Branch:       "main",
		EnableCommit: true,
		CommitID:     "new-sha",
	}}

	got := applyRepos(base, input)

	if !got[0].EnableCommit {
		t.Fatalf("expected commit mode to remain enabled")
	}
	if got[0].CommitID != "new-sha" {
		t.Fatalf("expected commit ID to be preserved, got %q", got[0].CommitID)
	}
}
