package service

type DeliveryArtifact struct {
	Name                string `json:"name"`
	Type                string `json:"type"`
	Source              string `json:"source"`
	Image               string `json:"image,omitempty"`
	ImageHash           string `json:"image_hash,omitempty"`
	ImageTag            string `json:"image_tag"`
	ImageDigest         string `json:"image_digest,omitempty"`
	ImageSize           int64  `json:"image_size,omitempty"`
	Architecture        string `json:"architecture,omitempty"`
	Os                  string `json:"os,omitempty"`
	PackageFileLocation string `json:"package_file_location,omitempty"`
	PackageStorageURI   string `json:"package_storage_uri,omitempty"`
	CreatedBy           string `json:"created_by"`
	CreatedTime         int64  `json:"created_time"`
}

type DeliveryActivity struct {
	Type              string            `json:"type"`
	Content           string            `json:"content,omitempty"`
	URL               string            `json:"url,omitempty"`
	Commits           []*ActivityCommit `json:"commits,omitempty"`
	Issues            []string          `json:"issues,omitempty"`
	Namespace         string            `json:"namespace,omitempty"`
	EnvName           string            `json:"env_name,omitempty"`
	PublishHosts      []string          `json:"publish_hosts,omitempty"`
	PublishNamespaces []string          `json:"publish_namespaces,omitempty"`
	RemoteFileKey     string            `json:"remote_file_key,omitempty"`
	DistStorageURL    string            `json:"dist_storage_url,omitempty"`
	SrcStorageURL     string            `json:"src_storage_url,omitempty"`
	StartTime         int64             `json:"start_time,omitempty"`
	EndTime           int64             `json:"end_time,omitempty"`
	CreatedBy         string            `json:"created_by"`
	CreatedTime       int64             `json:"created_time"`
}

type ActivityCommit struct {
	Address       string `json:"address"`
	Source        string `json:"source,omitempty"`
	RepoOwner     string `json:"repo_owner"`
	RepoName      string `json:"repo_name"`
	Branch        string `json:"branch"`
	PR            int    `json:"pr,omitempty"`
	Tag           string `json:"tag,omitempty"`
	CommitID      string `json:"commit_id,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
	AuthorName    string `json:"author_name,omitempty"`
}

type DeliveryArtifactInfo struct {
	DeliveryArtifact      *DeliveryArtifact              `json:"delivery_artifact"`
	DeliveryActivities    []*DeliveryActivity            `json:"activities"`
	DeliveryActivitiesMap map[string][]*DeliveryActivity `json:"sortedActivities,omitempty"`
}
