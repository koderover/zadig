package types

//PolicyMetaScope resource scope for permission
type PolicyMetaScope string

const (
	SystemScope  PolicyMetaScope = "system"
	ProjectScope PolicyMetaScope = "project"
)
