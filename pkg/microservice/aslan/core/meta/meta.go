package meta

import (
	_ "embed"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/tool/log"
)

//go:embed env_meta.yaml
var envMetaBytes []byte

//go:embed workflow_meta.yaml
var workflowPolicyMeta []byte

//go:embed other_metas.yaml
var otherPolicyMeta []byte

type MetaGetter interface {
	Policies() []*policy.PolicyMeta
}

type MetaWorkflow struct {
}

func (m *MetaWorkflow) Policies() []*policy.PolicyMeta {
	res := &policy.PolicyMeta{}
	err := yaml.Unmarshal(workflowPolicyMeta, res)
	if err != nil {
		// should not have happened here
		log.DPanic(err)
	}
	for i, meta := range res.Rules {
		tmpRules := []*policy.ActionRule{}
		for _, rule := range meta.Rules {
			if rule.ResourceType == "" {
				rule.ResourceType = res.Resource
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
		res.Rules[i].Rules = tmpRules
	}
	return []*policy.PolicyMeta{
		res,
	}
}

type MetaEnvironment struct {
}

func (m *MetaEnvironment) Policies() []*policy.PolicyMeta {
	envMeta := &policy.PolicyMeta{}
	err := yaml.Unmarshal(envMetaBytes, envMeta)
	if err != nil {
		// should not have happened here
		log.DPanic(err)
	}
	for i, meta := range envMeta.Rules {
		tmpRules := []*policy.ActionRule{}
		for _, rule := range meta.Rules {
			if rule.ResourceType == "" {
				rule.ResourceType = envMeta.Resource
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
		envMeta.Rules[i].Rules = tmpRules
	}

	proEnvMeta := &policy.PolicyMeta{}
	err = yaml.Unmarshal(envMetaBytes, proEnvMeta)
	if err != nil {
		// should not have happened here
		log.DPanic(err)
	}

	proEnvMeta.Resource = "ProductionEnvironment"
	proEnvMeta.Alias = "环境(生产/预发布)"
	for i, meta := range proEnvMeta.Rules {
		tmpRules := []*policy.ActionRule{}
		for _, rule := range meta.Rules {
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
						Value: "true",
					},
				}
			}

			tmpRules = append(tmpRules, rule)
		}
		proEnvMeta.Rules[i].Rules = tmpRules
	}

	return []*policy.PolicyMeta{envMeta, proEnvMeta}
}

type MetaOthers struct {
}

func (m *MetaOthers) Policies() []*policy.PolicyMeta {
	otherMetas := []*policy.PolicyMeta{}
	if err := yaml.Unmarshal(otherPolicyMeta, &otherMetas); err != nil {
		log.DPanic(err)
	}
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
	all.metaGetters = append(all.metaGetters, new(MetaOthers), new(MetaSystem), new(MetaWorkflow), new(MetaEnvironment))
	return all
}
