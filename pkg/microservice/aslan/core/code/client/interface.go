package client

type CodeHostClient interface {
	ListBranches(opt ListOpt) ([]*Branch, error)
	ListTags(opt ListOpt) ([]*Tag, error)
	ListPrs(opt ListOpt) ([]*PullRequest, error)
}

type ListOpt struct {
	Namespace   string
	ProjectName string
	Key         string
	Page        int
	PerPage     int
	TargeBr     string
}

type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
	Merged    bool   `json:"merged"`
}

type Tag struct {
	Name       string `json:"name"`
	ZipballURL string `json:"zipball_url"`
	TarballURL string `json:"tarball_url"`
	Message    string `json:"message"`
}

type PullRequest struct {
	ID             int    `json:"id"`
	TargetBranch   string `json:"targetBranch"`
	SourceBranch   string `json:"sourceBranch"`
	ProjectID      int    `json:"projectId"`
	Title          string `json:"title"`
	State          string `json:"state"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
	AuthorUsername string `json:"authorUsername"`
	Number         int    `json:"number"`
	User           string `json:"user"`
	Base           string `json:"base,omitempty"`
}
