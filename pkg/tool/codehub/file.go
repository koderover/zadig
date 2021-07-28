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

type FileContentResp struct {
	Result FileContentResult `json:"result"`
	Status string            `json:"status"`
}

type FileContentResult struct {
	FileName string `json:"file_name"`
	FilePath string `json:"file_path"`
	Size     string `json:"size"`
	Encoding string `json:"encoding"`
	Ref      string `json:"ref"`
	BlobID   string `json:"blob_id"`
	FileType string `json:"file_type"`
	Content  string `json:"content"`
}

type FileContent struct {
	FileName string `json:"file_name"`
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (c *CodeHubClient) FileContent(repoUUID, branchName, path string) (*FileContent, error) {
	fileContentResp := new(FileContentResp)
	body, err := c.sendRequest("GET", fmt.Sprintf("/v1/repositories/%s/branch/%s/file?path=%s", repoUUID, branchName, path), []byte{})
	if err != nil {
		return nil, err
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(fileContentResp); err != nil {
		return nil, err
	}

	if fileContentResp.Status == "success" {
		return &FileContent{
			FileName: fileContentResp.Result.FileName,
			FilePath: fileContentResp.Result.FilePath,
			Content:  fileContentResp.Result.Content,
		}, nil
	}

	return nil, fmt.Errorf("get file content failed")
}
