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

var defaultMetaGetter MetaGetter

func init() {
	defaultMetaGetter = newDefaultMetaGetter()
}

func DefaultPolicyMetas() []*types.PolicyMeta {
	return defaultMetaGetter.Policies()
}

func PolicyMetasFromBytes(bs []byte) []*types.PolicyMeta {
	return newMetaGetter(bs).Policies()
}

type MetaGetter interface {
	Policies() []*types.PolicyMeta
}

type MetaFromEmbedData struct {
	Metas []*types.PolicyMeta
}

func (m *MetaFromEmbedData) Policies() []*types.PolicyMeta {
	m.Metas = processMetas(m.Metas)
	return m.Metas
}

type MetaAllConfig struct {
	Metas []*types.PolicyMeta
	Bytes []byte
}

func (m *MetaAllConfig) Policies() []*types.PolicyMeta {
	m.Metas = processMetas(m.Metas)
	return m.Metas
}

func newMetaGetter(b []byte) MetaGetter {
	m := &MetaAllConfig{Bytes: b}
	if err := yaml.Unmarshal(m.Bytes, &m.Metas); err != nil {
		log.Warnf("fail to new meta getter err:%s, use the default meta getter", err)
		return defaultMetaGetter
	}
	return m
}

func newDefaultMetaGetter() MetaGetter {
	m := &MetaFromEmbedData{}
	if err := yaml.Unmarshal(PolicyMetas, &m.Metas); err != nil {
		log.DPanic(err)
	}
	return m
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
