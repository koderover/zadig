package codehub

import (
	"encoding/json"
	"fmt"
)

type TagList struct {
	Result struct {
		Total int `json:"total"`
		Tags  []struct {
			Name         string `json:"name"`
			IsDoubleName bool   `json:"is_double_name"`
		} `json:"tags"`
	} `json:"result"`
	Status string `json:"status"`
}

type Tag struct {
	Name       string `json:"name"`
	ZipballURL string `json:"zipball_url"`
	TarballURL string `json:"tarball_url"`
	Message    string `json:"message"`
}

func (c *CodeHubClient) TagList(repoID string) ([]*Tag, error) {
	tagInfos := make([]*Tag, 0)

	tagList := new(TagList)
	body, err := c.sendRequest("GET", fmt.Sprintf("/v2/repositories/%s/tags", repoID), "")
	if err != nil {
		return tagInfos, err
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(tagList); err != nil {
		return tagInfos, err
	}

	if tagList.Result.Total == 0 {
		return tagInfos, nil
	}

	for _, tag := range tagList.Result.Tags {
		tagInfos = append(tagInfos, &Tag{
			Name: tag.Name,
		})
	}
	return tagInfos, nil
}
