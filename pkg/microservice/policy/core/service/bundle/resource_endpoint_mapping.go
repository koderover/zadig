package bundle

import (
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
)

type resourceActionMappings map[string]map[string][]*rule

func (m resourceActionMappings) GetRules(resource string, actions []string) []*rule {
	mappings, ok := m[resource]
	if !ok {
		return nil
	}

	all := false
	if len(actions) == 1 && actions[0] == models.MethodAll {
		all = true
	}
	actionSet := sets.NewString(actions...)
	var res []*rule
	for action, r := range mappings {
		if all || actionSet.Has(action) {
			res = append(res, r...)
		}
	}

	return res
}

//var resourceActionMappings = map[string]map[string][]*rule{
//	"Workflow": workflowMapping,
//}
//
//var workflowMapping = map[string][]*rule{
//	ActionCreate: {
//		{Method: MethodPost, Endpoint: "/api/aslan/workflow/workflow"},
//		{Method: MethodPost, Endpoint: "/api/aslan/workflow/v2/pipelines"},
//	},
//	ActionDelete: {
//		{Method: MethodPost, Endpoint: "/api/aslan/workflow/workflow/?*"},
//		{Method: MethodPost, Endpoint: "/api/aslan/workflow/v2/pipelines/?*"},
//	},
//}

func getResourceActionMappings(policies []*models.Policy) resourceActionMappings {
	data := make(resourceActionMappings)
	for _, p := range policies {
		if _, ok := data[p.Resource]; !ok {
			data[p.Resource] = make(map[string][]*rule)
		}

		for _, r := range p.Rules {
			for _, ar := range r.Rules {
				data[p.Resource][r.Action] = append(data[p.Resource][r.Action], &rule{Method: ar.Method, Endpoint: ar.Endpoint})
			}
		}
	}

	return data
}
