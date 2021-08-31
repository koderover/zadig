package ilyshin

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Commit struct {
	ID             string     `json:"id"`
	ShortID        string     `json:"short_id"`
	Title          string     `json:"title"`
	Message        string     `json:"message"`
	AuthorName     string     `json:"author_name"`
	AuthorEmail    string     `json:"author_email"`
	AuthoredDate   *time.Time `json:"authored_date"`
	CommitterName  string     `json:"committer_name"`
	CommitterEmail string     `json:"committer_email"`
	CommittedDate  *time.Time `json:"committed_date"`
	CreatedAt      *time.Time `json:"created_at"`
}

func (c *Client) GetLatestCommit(owner, repo, branch, path string, log *zap.SugaredLogger) (*Commit, error) {
	url := fmt.Sprintf("/api/v4/projects/%s/repository/commits", generateProjectName(owner, repo))
	qs := map[string]string{
		"per_page": "1",
		"ref_name": branch,
		"path":     path,
	}

	var err error
	var commits []*Commit
	if _, err = c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&commits)); err != nil {
		log.Errorf("Failed to list project commits, error: %s", err)
		return nil, err
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("not found commits")
	}

	return commits[0], nil
}
