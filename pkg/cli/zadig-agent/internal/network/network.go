/*
Copyright 2023 The KodeRover Authors.

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

package network

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
	httpclient "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/client"
)

type AgentConfig struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}

func GetFullURL(host, base string) string {
	return fmt.Sprintf("%s%s", host, base)
}

type ZadigClient struct {
	AgentConfig *AgentConfig
	Config      *config.AgentConfig
}

func NewZadigClient() *ZadigClient {
	return &ZadigClient{
		AgentConfig: &AgentConfig{
			Token: config.GetAgentToken(),
			URL:   config.GetServerURL(),
		},
	}
}

func (c *ZadigClient) RequestJob() (*types.ZadigJobTask, error) {
	resp := new(types.ZadigJobTask)

	body, err := httpclient.Get(GetFullURL(c.AgentConfig.URL, RequestJobBaseUrl), httpclient.SetQueryParam("token", c.AgentConfig.Token), httpclient.SetResult(resp))
	if err != nil {
		return nil, err
	}

	if body.IsError() {
		errMsg := httpclient.NewErrorFromRestyResponse(body)
		return nil, errMsg
	}

	if body.IsSuccess() {
		return resp, nil
	}
	return nil, fmt.Errorf("failed to request job from zadig server")
}

type ReportJobRequest struct {
	Token     string `json:"token"`
	JobID     string `json:"job_id"`
	JobStatus string `json:"job_status"`
	JobLog    string `json:"job_log"`
	JobError  string `json:"job_error"`
	JobOutput []byte `json:"job_output"`
	Seq       int    `json:"seq"`
}

func (c *ZadigClient) ReportJob(parameters *types.ReportJobParameters) (*types.ReportAgentJobResp, error) {
	request := &ReportJobRequest{
		Token: c.AgentConfig.Token,
	}
	if parameters != nil {
		request.JobID = parameters.JobID
		request.JobStatus = parameters.JobStatus
		request.JobLog = parameters.JobLog
		request.JobError = parameters.JobError
		request.JobOutput = parameters.JobOutput
		request.Seq = parameters.Seq
	}
	resp := new(types.ReportAgentJobResp)

	body, err := httpclient.Post(GetFullURL(c.AgentConfig.URL, ReportJobBaseUrl), httpclient.SetBody(request), httpclient.SetResult(resp))
	if err != nil {
		return nil, err
	}

	if body.IsError() {
		err := httpclient.NewErrorFromRestyResponse(body)
		return nil, err
	}

	if body.IsSuccess() {
		return resp, nil
	}
	return nil, fmt.Errorf("failed to report job to zadig server")
}

type DownloadFileRequest struct {
	Token string `json:"token"`
}

// DownloadFile downloads a file from the server using the file ID
func (c *ZadigClient) DownloadFile(fileID, targetPath string) error {
	downloadURL := GetFullURL(c.AgentConfig.URL, fmt.Sprintf(DownloadFileBaseUrl+"?token=%s", fileID, c.AgentConfig.Token))

	err := httpclient.Download(downloadURL, targetPath)
	if err != nil {
		return fmt.Errorf("failed to download file (ID: %s) to %s: %v", fileID, targetPath, err)
	}

	return nil
}
