package client

type CodeHostClient interface {
	ListBranches(namespace, projectName, key string, page, perPage int) ([]*Branch, error)
	ListTags() ([]*Tag, error)
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
