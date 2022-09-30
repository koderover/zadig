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
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util/deepcopy"
)

//go:embed metas.yaml
var PolicyMetas []byte

var defaultMetaConfig *MetaConfig

func init() {
	log.Init(&log.Config{
		Level:    "debug",
		NoCaller: true,
	})
	defaultMetaConfig = newMetaConfigFromEmbed()
}

func DefaultPolicyMetas() []*types.PolicyMeta {
	return defaultMetaConfig.Policies()
}

func DefaultPolicyMetasConfig() *MetaConfig {
	return defaultMetaConfig
}

func PolicyMetasFromBytes(bs []byte) []*types.PolicyMeta {
	return newMetaConfigFromBytes(bs).Policies()
}

type MetaGetter interface {
	Policies() []*types.PolicyMeta
}

type MetaConfig struct {
	Description string              `json:"description"`
	Metas       []*types.PolicyMeta `json:"metas"`
}

func (m *MetaConfig) Policies() []*types.PolicyMeta {
	m.Metas = processMetas(m.Metas)
	return m.Metas
}

func newMetaConfigFromBytes(b []byte) MetaGetter {
	m := &MetaConfig{}
	if err := yaml.Unmarshal(b, &m); err != nil {
		log.Warnf("fail to new meta getter err:%s, use the default meta getter", err)
		return defaultMetaConfig
	}
	return m
}

func newMetaConfigFromEmbed() *MetaConfig {
	config := &MetaConfig{}
	if err := yaml.Unmarshal(PolicyMetas, &config); err != nil {
		log.DPanic(err)
	}
	return config
}

func processMetas(metas []*types.PolicyMeta) []*types.PolicyMeta {
	proEnvMeta := &types.PolicyMeta{}

	for _, meta := range metas {
		if meta.Resource == "Workflow" {
			for _, ruleMeta := range meta.Rules {
				tmpRules := []*types.ActionRule{}
				for _, rule := range ruleMeta.Rules {
					if rule.ResourceType == "" {
						rule.ResourceType = "Workflow"
					}
					if strings.Contains(rule.Endpoint, ":name") {
						idRegex := strings.ReplaceAll(rule.Endpoint, ":name", `([\w\W].*)`)
						idRegex = strings.ReplaceAll(idRegex, "?*", `[\w\W].*`)
						endpoint := strings.ReplaceAll(rule.Endpoint, ":name", "?*")
						rule.Endpoint = endpoint
						rule.IDRegex = idRegex
					}
					if rule.Filter {
						rule.MatchAttributes = []*types.Attribute{
							{
								Key:   "placeholder",
								Value: "placeholder",
							},
						}
					}
					tmpRules = append(tmpRules, rule)
				}
				ruleMeta.Rules = tmpRules
			}
		}
		if meta.Resource == "Environment" {
			for _, ruleMeta := range meta.Rules {
				tmpRules := []*types.ActionRule{}
				for _, rule := range ruleMeta.Rules {
					if rule.ResourceType == "" {
						rule.ResourceType = "Environment"
					}
					if strings.Contains(rule.Endpoint, ":name") {
						idRegex := strings.ReplaceAll(rule.Endpoint, ":name", `([\w\W].*)`)
						idRegex = strings.ReplaceAll(idRegex, "?*", `[\w\W].*`)
						endpoint := strings.ReplaceAll(rule.Endpoint, ":name", "?*")
						rule.Endpoint = endpoint
						rule.IDRegex = idRegex
					}

					tmpRules = append(tmpRules, rule)
				}
				ruleMeta.Rules = tmpRules
			}

			if err := deepcopy.FromTo(meta, proEnvMeta); err != nil {
				log.DPanic(err)
			}
		}
	}
	metas = append(metas, proEnvMeta)
	return metas
}
