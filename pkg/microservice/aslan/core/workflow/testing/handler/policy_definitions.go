package handler

import (
	_ "embed"

	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/tool/log"
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

	return []*policy.Policy{res}
}
