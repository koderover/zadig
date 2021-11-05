package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
)

type Policy struct {
	Resource    string        `json:"resource"`
	Alias       string        `json:"alias"`
	Description string        `json:"description"`
	Rules       []*PolicyRule `json:"rules"`
}

type PolicyRule struct {
	Action      string        `json:"action"`
	Alias       string        `json:"alias"`
	Description string        `json:"description"`
	Rules       []*ActionRule `json:"rules"`
}

type ActionRule struct {
	Method   string `json:"method"`
	Endpoint string `json:"endpoint"`
}

type PolicyDefinition struct {
	Resource    string                  `json:"resource"`
	Alias       string                  `json:"alias"`
	Description string                  `json:"description"`
	Rules       []*PolicyRuleDefinition `json:"rules"`
}

type PolicyRuleDefinition struct {
	Action      string `json:"action"`
	Alias       string `json:"alias"`
	Description string `json:"description"`
}

func CreateOrUpdatePolicyRegistration(p *Policy, _ *zap.SugaredLogger) error {
	obj := &models.Policy{
		Resource:    p.Resource,
		Alias:       p.Alias,
		Description: p.Description,
	}

	for _, r := range p.Rules {
		rule := &models.PolicyRule{
			Action:      r.Action,
			Alias:       r.Alias,
			Description: r.Description,
		}
		for _, ar := range r.Rules {
			rule.Rules = append(rule.Rules, &models.ActionRule{Method: ar.Method, Endpoint: ar.Endpoint})
		}
		obj.Rules = append(obj.Rules, rule)
	}

	return mongodb.NewPolicyColl().UpdateOrCreate(obj)
}

func GetPolicyRegistrationDefinitions(_ *zap.SugaredLogger) ([]*PolicyDefinition, error) {
	policies, err := mongodb.NewPolicyColl().List()
	if err != nil {
		return nil, err
	}

	var res []*PolicyDefinition
	for _, p := range policies {
		pd := &PolicyDefinition{
			Resource:    p.Resource,
			Alias:       p.Alias,
			Description: p.Description,
		}
		for _, r := range p.Rules {
			pd.Rules = append(pd.Rules, &PolicyRuleDefinition{
				Action:      r.Action,
				Alias:       r.Alias,
				Description: r.Description,
			})
		}

		res = append(res, pd)
	}

	return res, nil
}
