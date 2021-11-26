package resource

type Node struct {
	Labels []string `json:"node_labels"`
	Status string   `json:"node_status"`
}
