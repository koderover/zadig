package codehub

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/util/sets"
)

type RepoListInfo struct {
	Result struct {
		Total        int `json:"total"`
		Repositories []struct {
			Star             bool        `json:"star"`
			Status           int         `json:"status"`
			UserRole         interface{} `json:"userRole"`
			RepositoryUUID   string      `json:"repository_uuid"`
			RepositoryID     int         `json:"repository_id"`
			RepositoryName   string      `json:"repository_name"`
			SSHURL           string      `json:"ssh_url"`
			HTTPSURL         string      `json:"https_url"`
			GroupName        string      `json:"group_name"`
			WebURL           string      `json:"web_url"`
			VisibilityLevel  int         `json:"visibility_level"`
			CreatedAt        string      `json:"created_at"`
			UpdatedAt        string      `json:"updated_at"`
			RepositorySize   string      `json:"repository_size"`
			LfsSize          string      `json:"lfs_size"`
			CreatorName      string      `json:"creator_name"`
			DomainName       string      `json:"domain_name"`
			IsOwner          int         `json:"is_owner"`
			IamUserUUID      string      `json:"iam_user_uuid"`
			ProjectUUID      string      `json:"project_uuid"`
			ProjectIsDeleted string      `json:"project_is_deleted"`
		} `json:"repositories"`
	} `json:"result"`
	Status string `json:"status"`
}

type Namespace struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	ProjectUUID string `json:"project_uuid,omitempty"`
}

func (c *CodeHubClient) NamespaceList() ([]*Namespace, error) {
	groupInfos := make([]*Namespace, 0)
	repoListInfo := new(RepoListInfo)
	body, err := c.sendRequest("GET", "/v2/projects/repositories", "")
	if err != nil {
		return groupInfos, err
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(repoListInfo); err != nil {
		return groupInfos, err
	}
	if repoListInfo.Result.Total == 0 {
		return groupInfos, nil
	}
	groupNames := sets.NewString()
	for _, repository := range repoListInfo.Result.Repositories {
		if repository.ProjectIsDeleted == "true" {
			continue
		}
		if groupNames.Has(repository.GroupName) {
			continue
		}
		groupNames.Insert(repository.GroupName)
		groupInfos = append(groupInfos, &Namespace{
			Name:        repository.GroupName,
			Path:        repository.CreatorName,
			ProjectUUID: repository.ProjectUUID,
			Kind:        "user",
		})
	}
	return groupInfos, nil
}
