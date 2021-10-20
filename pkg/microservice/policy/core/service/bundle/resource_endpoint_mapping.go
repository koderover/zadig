package bundle

var mappings = map[string]map[string][]*rule{
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
