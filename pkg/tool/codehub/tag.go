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

package codehub

import (
	"encoding/json"
	"fmt"
)

type TagList struct {
	Result TagListResult `json:"result"`
	Status string        `json:"status"`
}

type TagListResult struct {
	Total int    `json:"total"`
	Tags  []Tags `json:"tags"`
}

type Tags struct {
	Name         string `json:"name"`
	IsDoubleName bool   `json:"is_double_name"`
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
	body, err := c.sendRequest("GET", fmt.Sprintf("/v2/repositories/%s/tags", repoID), []byte{})
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
