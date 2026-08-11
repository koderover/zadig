package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/koderover/zadig/v2/pkg/types"
)

func TestOpenAPICreateTestTaskReqValidateRepositoryRef(t *testing.T) {
	tests := []struct {
		name      string
		repo      *types.OpenAPIRepoInput
		wantValid bool
		wantError string
	}{
		{
			name: "branch ref",
			repo: &types.OpenAPIRepoInput{
				CodeHostName:  "github",
				RepoNamespace: "koderover",
				RepoName:      "zadig",
				Branch:        "main",
			},
			wantValid: true,
		},
		{
			name: "commit ref without branch",
			repo: &types.OpenAPIRepoInput{
				CodeHostName:  "github",
				RepoNamespace: "koderover",
				RepoName:      "zadig",
				EnableCommit:  true,
				CommitID:      "abc123",
			},
			wantValid: true,
		},
		{
			name: "commit ref without commit id",
			repo: &types.OpenAPIRepoInput{
				CodeHostName:  "github",
				RepoNamespace: "koderover",
				RepoName:      "zadig",
				Branch:        "main",
				EnableCommit:  true,
			},
			wantError: "repo_info[0] commit_id cannot be empty when enable_commit is true",
		},
		{
			name: "branch ref without branch",
			repo: &types.OpenAPIRepoInput{
				CodeHostName:  "github",
				RepoNamespace: "koderover",
				RepoName:      "zadig",
			},
			wantError: "repo_info[0] branch cannot be empty when enable_commit is false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &OpenAPICreateTestTaskReq{
				ProjectName: "project",
				TestName:    "test",
				RepoInfo:    []*types.OpenAPIRepoInput{tt.repo},
			}

			valid, err := req.Validate()
			assert.Equal(t, tt.wantValid, valid)
			if tt.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantError)
		})
	}
}
