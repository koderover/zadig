package codehub

import (
	"encoding/json"
	"fmt"
)

type FileContent struct {
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

func (c *CodeHubClient) FileContent(repoUUID, branchName, path string) (*FileContent, error) {
	fileContent := new(FileContent)
	body, err := c.sendRequest("GET", fmt.Sprintf("/v1/repositories/%s/branch/%s/file?path=%s", repoUUID, branchName, path), "")
	if err != nil {
		return fileContent, err
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(fileContent); err != nil {
		return fileContent, err
	}

	if fileContent.Status == "success" {
		return fileContent, nil
	}

	return nil, fmt.Errorf("reponse failed")
}
