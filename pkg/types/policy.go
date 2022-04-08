package types

//PolicyMetaResourceScope resource scope for permission
type PolicyMetaResourceScope string

const (
	ClusterScope PolicyMetaResourceScope = "cluster"
	ProjectScope PolicyMetaResourceScope = "project"
)
