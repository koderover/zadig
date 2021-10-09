package bundle

type exemptionURLs struct {
	Global     rules `json:"global"`
	Namespaced rules `json:"namespaced"`
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
