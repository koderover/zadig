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

type MetaGetter interface {
	Policies() []*types.PolicyMeta
}

type MetaOthers struct {
}

func (m *MetaOthers) Policies() []*types.PolicyMeta {
	policyMetas := []*types.PolicyMeta{}
	if err := yaml.Unmarshal(PolicyMetas, &policyMetas); err != nil {
		log.DPanic(err)
	}

	proEnvMeta := &types.PolicyMeta{}

	for _, meta := range policyMetas {
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
	policyMetas = append(policyMetas, proEnvMeta)
	return policyMetas
}

type MetaAll struct {
	metaGetters []MetaGetter
}

func (m *MetaAll) Policies() []*types.PolicyMeta {
	all := []*types.PolicyMeta{}
	for _, v := range m.metaGetters {
		all = append(all, v.Policies()...)
	}
	return all
}

func NewMetaGetter() MetaGetter {
	all := &MetaAll{}
	all.metaGetters = append(all.metaGetters, new(MetaOthers))
	return all
}
