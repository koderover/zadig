package bundle

// TODO: Policy Service should not care about the policy details of a certain service, instead, a service which wants
// to be managed by the Policy Service should register its rules to the Policy Service so that the Policy Service knows
// how to make a decision without being aware of the detailed rules.

type exemptionURLs struct {
	Global     rules `json:"global"`     // global urls are only controlled by AuthN, and it is visible for all users
	Namespaced rules `json:"namespaced"` // global urls are only controlled by AuthN, and it is visible for users under certain projects
	Public     rules `json:"public"`     // public urls are not controlled by AuthN and AuthZ
}

type policyRule struct {
	Methods   []string `json:"methods"`
	Endpoints []string `json:"endpoints"`
}

var globalURLs = []*policyRule{
	{
		Methods:   []string{"*"},
		Endpoints: []string{"api/v1/version"},
	},
}

var namespacedURLs = []*policyRule{
	{
		Methods:   []string{"*"},
		Endpoints: []string{"api/v1/version1"},
	},
}

var publicURLs = []*policyRule{
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/health"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/webhook"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/hub/connect"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/kodespace/downloadUrl"},
	},
}
