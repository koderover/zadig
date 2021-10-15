package bundle

// TODO: Policy Service should not care about the policy details of a certain service, instead, a service which wants
// to be managed by the Policy Service should register its rules to the Policy Service so that the Policy Service knows
// how to make a decision without being aware of the detailed rules.

var resourceActionMappings = map[string]map[string][]*rule{
	"Workflow": workflowMapping,
}

var workflowMapping = map[string][]*rule{
	ActionCreate: {
		{Method: MethodPost, Endpoint: "/api/aslan/workflow/workflow"},
		{Method: MethodPost, Endpoint: "/api/aslan/workflow/v2/pipelines"},
	},
	ActionDelete: {
		{Method: MethodPost, Endpoint: "/api/aslan/workflow/workflow/?*"},
		{Method: MethodPost, Endpoint: "/api/aslan/workflow/v2/pipelines/?*"},
	},
}
