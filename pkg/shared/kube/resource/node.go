package resource

type Node struct {
	Labels []string `json:"node_labels"`
	Status string   `json:"node_status"`
	IP     string   `json:"node_ip"`
}

type NodeResp struct {
	Nodes  []*Node  `json:"data"`
	Labels []string `json:"labels"`
}
