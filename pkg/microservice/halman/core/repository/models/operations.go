package models

type ProjectInput struct {
	Key         string            `json:"product_name"`
	Name        string            `json:"project_name"`
	Description string            `json:"desc"`
	Group       string            `json:"group_name"`
	Admins      []string          `json:"admins"`
	Public      bool              `json:"public"`
	Enabled     bool              `json:"enabled"`
	Feature     map[string]string `json:"product_feature"`
}
