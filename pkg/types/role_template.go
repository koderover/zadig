package types

type RoleTemplate struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"desc"`
	Type        string `json:"type"`
}

type DetailedRoleTemplate struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Description string `json:"desc"`
	// ResourceActions represents a set of verbs with its corresponding resource.
	// the json response of this field `rules` is used for compatibility.
	ResourceActions []*ResourceAction `json:"rules"`
}
