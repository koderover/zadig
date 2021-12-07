package resource

type Node struct {
	Labels []string `json:"node_labels"`
	Ready  bool     `json:"ready"`
	IP     string   `json:"node_ip"`
}

type NodeResp struct {
	Nodes  []*Node  `json:"data"`
	Labels []string `json:"labels"`
}
