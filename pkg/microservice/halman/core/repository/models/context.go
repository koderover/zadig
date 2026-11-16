package models

type ContextScope struct {
	ID      string
	Context PageContext
	Scoped  bool
	Scopes  []ResourceScope
}

type ResourceScope struct {
	PageContext
	Catalog      string `json:"catalog,omitempty"`
	SystemAction string `json:"system_action,omitempty"`
}
