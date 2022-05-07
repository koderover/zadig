package meta

import (
	_ "embed"
	"strings"

	"github.com/qdm12/reprint"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/tool/log"
)

//go:embed metas.yaml
var PolicyMetas []byte

type MetaGetter interface {
	Policies() []*policy.PolicyMeta
}

type MetaOthers struct {
}

func (m *MetaOthers) Policies() []*policy.PolicyMeta {
	policyMetas := []*policy.PolicyMeta{}
	if err := yaml.Unmarshal(PolicyMetas, &policyMetas); err != nil {
		log.DPanic(err)
	}

	proEnvMeta := &policy.PolicyMeta{}

	for _, meta := range policyMetas {
		if meta.Resource == "Workflow" {
			for _, ruleMeta := range meta.Rules {
				tmpRules := []*policy.ActionRule{}
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
				tmpRules := []*policy.ActionRule{}
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
						rule.MatchAttributes = []*policy.Attribute{
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

func (m *MetaAll) Policies() []*policy.PolicyMeta {
	all := []*policy.PolicyMeta{}
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
