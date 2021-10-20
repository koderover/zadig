package bundle

// TODO: Policy Service should not care about the policy details of a certain service, instead, a service which wants
// to be managed by the Policy Service should register its rules to the Policy Service so that the Policy Service knows
// how to make a decision without being aware of the detailed rules.

type exemptionURLs struct {
	Global     rules `json:"global"`     // global urls are only controlled by AuthN, and it is visible for all users
	Namespaced rules `json:"namespaced"` // global urls are only controlled by AuthN, and it is visible for users under certain projects
	Public     rules `json:"public"`     // public urls are not controlled by AuthN and AuthZ
	Registered rules `json:"registered"` // registered urls are the entire list of urls which are controlled by AuthZ, which means that if an url is not in this list, it is not controlled by AuthZ
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
		Endpoints: []string{"api/aslan/testing/report"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/cluster/agent/?*/agent.yaml"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/podexec/health"},
	},
}

// actions which are allowed for admins only.
var adminURLs = []*policyRule{
	{
		Methods:   []string{"POST", "PUT"},
		Endpoints: []string{"api/aslan/project/products"},
	},
	{
		Methods:   []string{"DELETE"},
		Endpoints: []string{"api/aslan/project/products/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/cluster/clusters"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/cluster/clusters/?*"},
	},
	{
		Methods:   []string{"PUT"},
		Endpoints: []string{"api/aslan/cluster/clusters/?*/disconnect"},
	},
	{
		Methods:   []string{"PUT"},
		Endpoints: []string{"api/aslan/cluster/clusters/?*/reconnect"},
	},
	{
		Methods:   []string{"POST", "PUT"},
		Endpoints: []string{"api/aslan/system/install"},
	},
	{
		Methods:   []string{"PUT"},
		Endpoints: []string{"api/aslan/system/install/delete"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/proxyManage"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/proxyManage/?*"},
	},
	{
		Methods:   []string{"GET", "POST"},
		Endpoints: []string{"api/aslan/system/registry/namespaces"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/registry/namespaces/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/s3storage"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/s3storage/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/githubApp"},
	},
	{
		Methods:   []string{"DELETE"},
		Endpoints: []string{"api/aslan/system/githubApp/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/jenkins/integration"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/jenkins/integration/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/jenkins/integration/user/connection"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/basicImages"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/basicImages/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/helm"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/helm/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/privateKey"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/privateKey/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/announcement"},
	},
	{
		Methods:   []string{"PUT"},
		Endpoints: []string{"api/aslan/system/announcement/update"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/system/announcement/all"},
	},
	{
		Methods:   []string{"DELETE"},
		Endpoints: []string{"api/aslan/system/announcement/?*"},
	},
	{
		Methods:   []string{"GET", "POST"},
		Endpoints: []string{"api/aslan/system/operation"},
	},
	{
		Methods:   []string{"PUT"},
		Endpoints: []string{"api/aslan/system/operation/?*"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/system/proxy/config"},
	},
}
