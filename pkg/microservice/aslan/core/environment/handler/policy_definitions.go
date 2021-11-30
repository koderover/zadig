package handler

import (
	_ "embed"

	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/tool/log"
)

const (
	productionPolicyResource = "ProductionEnvironment"
	productionPolicyAlias    = "集成环境(类生产/预发布)"
	productionKey            = "production"
	productionValueTrue      = "true"
)

//go:embed policy.yaml
var policyDefinitions []byte

func (*Router) Policies() []*policy.Policy {
	res := &policy.Policy{}
	err := yaml.Unmarshal(policyDefinitions, res)
	if err != nil {
		// should not have happened here
		log.DPanic(err)
	}

	productionPolicy := &policy.Policy{}
	_ = yaml.Unmarshal(policyDefinitions, productionPolicy)
	productionPolicy.Resource = productionPolicyResource
	productionPolicy.Alias = productionPolicyAlias
	//productionPolicy.Description = ""
	for _, ru := range productionPolicy.Rules {
		for _, r := range ru.Rules {
			for _, a := range r.MatchAttributes {
				if a.Key == productionKey {
					a.Value = productionValueTrue
				}
			}
		}
	}

	return []*policy.Policy{res, productionPolicy}
}
