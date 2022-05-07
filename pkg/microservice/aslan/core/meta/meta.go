package meta

import (
	_ "embed"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/mitchellh/copystructure"

	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/tool/log"
)

//go:embed env_meta.yaml
var envMetaBytes []byte

//go:embed other_metas.yaml
var otherPolicyMeta []byte

type MetaGetter interface {
	Policies() []*policy.PolicyMeta
}

//type MetaEnvironment struct {
//}
//
//func (m *MetaEnvironment) Policies() []*policy.PolicyMeta {
//	envMeta := &policy.PolicyMeta{}
//	err := yaml.Unmarshal(envMetaBytes, envMeta)
//	if err != nil {
//		// should not have happened here
//		log.DPanic(err)
//	}
//	for i, meta := range envMeta.Rules {
//		tmpRules := []*policy.ActionRule{}
//		for _, rule := range meta.Rules {
//			if rule.ResourceType == "" {
//				rule.ResourceType = envMeta.Resource
//			}
//			if strings.Contains(rule.Endpoint, ":name") {
//				idRegex := strings.ReplaceAll(rule.Endpoint, ":name", `([\w\W].*)`)
//				idRegex = strings.ReplaceAll(idRegex, "?*", `[\w\W].*`)
//				endpoint := strings.ReplaceAll(rule.Endpoint, ":name", "?*")
//				rule.Endpoint = endpoint
//				rule.IDRegex = idRegex
//				rule.MatchAttributes = []*policy.Attribute{
//					{
//						Key:   "production",
//						Value: "false",
//					},
//				}
//			}
//
//			tmpRules = append(tmpRules, rule)
//		}
//		envMeta.Rules[i].Rules = tmpRules
//	}
//	proEnvMetaI, err := copystructure.Copy(envMeta)
//	if err != nil {
//		log.DPanic(err)
//	}
//	proEnvMeta, ok := proEnvMetaI.(*policy.PolicyMeta)
//	if !ok {
//		log.DPanic()
//	}
//
//	proEnvMeta.Resource = "ProductionEnvironment"
//	proEnvMeta.Alias = "环境(生产/预发布)"
//	for _, ru := range proEnvMeta.Rules {
//		for _, r := range ru.Rules {
//			for _, a := range r.MatchAttributes {
//				if a.Key == "production" {
//					a.Value = "true"
//				}
//			}
//		}
//	}
//	return []*policy.PolicyMeta{envMeta, proEnvMeta}
//}

type MetaOthers struct {
}

func (m *MetaOthers) Policies() []*policy.PolicyMeta {
	otherMetas := []*policy.PolicyMeta{}
	if err := yaml.Unmarshal(otherPolicyMeta, &otherMetas); err != nil {
		log.DPanic(err)
	}

	proEnvMeta := &policy.PolicyMeta{}

	for _, meta := range otherMetas {
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
			proEnvMetaI, err := copystructure.Copy(meta)
			if err != nil {
				log.DPanic(err)
			}
			proEnvMeta, ok := proEnvMetaI.(*policy.PolicyMeta)
			if !ok {
				log.DPanic()
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
	otherMetas = append(otherMetas, proEnvMeta)
	return otherMetas
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
