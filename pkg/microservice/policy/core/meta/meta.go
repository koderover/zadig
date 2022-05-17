package meta

import (
	_ "embed"
	"strings"

	"github.com/qdm12/reprint"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

//go:embed metas.yaml
var PolicyMetas []byte

var defaultMetaConfig *MetaConfig

func init() {
	defaultMetaConfig = newMetaConfigFromEmbed()
}

func DefaultPolicyMetas() []*types.PolicyMeta {
	return defaultMetaConfig.Policies()
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

type MetaConfigMap struct {
	Metas []*types.PolicyMeta
}

func (m *MetaConfigMap) Policies() []*types.PolicyMeta {
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
						rule.MatchAttributes = []*types.Attribute{
							{
								Key:   "production",
								Value: "false",
							},
						}
					}

					tmpRules = append(tmpRules, rule)
				}
				ruleMeta.Rules = tmpRules
			}

			if err := reprint.FromTo(meta, proEnvMeta); err != nil {
				log.DPanic(err)
			}
			proEnvMeta.Resource = "ProductionEnvironment"
			proEnvMeta.Alias = "环境(生产/预发布)"
			for _, ru := range proEnvMeta.Rules {
				for _, r := range ru.Rules {
					for _, a := range r.MatchAttributes {
						if a.Key == "production" {
							a.Value = "true"
						}
					}
				}
			}
		}
	}
	metas = append(metas, proEnvMeta)
	return metas
}
