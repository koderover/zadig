package service

type Rule struct {
	Verbs     []string `json:"verbs"`
	Resources []string `json:"resources"`
	Kind      string   `json:"kind"`
}

const SystemScope = "*"
