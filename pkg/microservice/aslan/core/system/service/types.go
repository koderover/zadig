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

package service

type ExternalSystemDetail struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Server   string `json:"server"`
	APIToken string `json:"api_token,omitempty"`
}

type WorkflowConcurrencySettings struct {
	WorkflowConcurrency int64 `json:"workflow_concurrency"`
	BuildConcurrency    int64 `json:"build_concurrency"`
}

type SonarIntegration struct {
	ID            string `json:"id"`
	ServerAddress string `json:"server_address"`
	Token         string `json:"token"`
}
