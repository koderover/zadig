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

import (
	"errors"
	"net/url"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
)

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

type OpenAPICreateRegistryReq struct {
	Address   string                  `json:"address"`
	Provider  config.RegistryProvider `json:"provider"`
	Namespace string                  `json:"namespace"`
	IsDefault bool                    `json:"is_default"`
	AccessKey string                  `json:"access_key"`
	SecretKey string                  `json:"secret_key"`
	EnableTLS bool                    `json:"enable_tls"`
	// Optional field below
	Region  string `json:"region"`
	TLSCert string `json:"tls_cert"`
}

func (req OpenAPICreateRegistryReq) Validate() error {
	if req.Address == "" {
		return errors.New("address cannot be empty")
	}

	switch req.Provider {
	case config.RegistryProviderECR, config.RegistryProviderDockerhub, config.RegistryProviderACR, config.RegistryProviderHarbor, config.RegistryProviderNative, config.RegistryProviderSWR, config.RegistryProviderTCR:
		break
	default:
		return errors.New("unsupported registry provider")
	}

	// only ECR can ignore namespace since it is in the address
	if req.Namespace == "" && req.Provider != config.RegistryProviderECR {
		return errors.New("namespace cannot be empty")
	}

	if req.Provider == config.RegistryProviderECR && req.Region == "" {
		return errors.New("region is a required field for ECR provider")
	}

	// address needs to be a validate url
	_, err := url.ParseRequestURI(req.Address)
	if err != nil {
		return errors.New("address needs to be a valid URL")
	}

	return nil
}
