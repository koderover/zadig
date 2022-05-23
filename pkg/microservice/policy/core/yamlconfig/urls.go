/*
Copyright 2022 The KodeRover Authors.

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

package yamlconfig

import (
	_ "embed"

	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

//go:embed urls.yaml
var urls []byte
var defaultEmbedUrlConfig *ConfigYaml
var configMapUrlConfig *ConfigYaml
var useConfigMapUrls bool

type ConfigYaml struct {
	ExemptionUrls *ExemptionURLs `json:"exemption_urls"`
	Description   string         `json:"description"`
}

type ExemptionURLs struct {
	Public       []*types.PolicyRule `json:"public"`
	SystemAdmin  []*types.PolicyRule `json:"system_admin"`
	ProjectAdmin []*types.PolicyRule `json:"project_admin"`
}

func init() {
	log.Init(&log.Config{
		Level:    "debug",
		NoCaller: true,
	})
	if err := yaml.Unmarshal(urls, &defaultEmbedUrlConfig); err != nil {
		log.DPanic(err)
	}
}

func GetExemptionsUrls() *ExemptionURLs {
	if !useConfigMapUrls || configMapUrlConfig == nil {
		return defaultEmbedUrlConfig.ExemptionUrls
	}
	return configMapUrlConfig.ExemptionUrls
}

func GetDefaultEmbedUrlConfig() *ConfigYaml {
	return defaultEmbedUrlConfig
}

func RefreshConfigMapByte(b []byte) error {
	if err := yaml.Unmarshal(b, &configMapUrlConfig); err != nil {
		log.Errorf("refresh err:%s", err)
		useConfigMapUrls = false
		return err
	}
	log.Infof("start to use configMap url")
	useConfigMapUrls = true
	return nil
}
