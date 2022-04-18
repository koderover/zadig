package types

//PolicyMetaResourceScope resource scope for permission
type PolicyMetaResourceScope string

const (
	SystemScope  PolicyMetaResourceScope = "system"
	ProjectScope PolicyMetaResourceScope = "project"
)
