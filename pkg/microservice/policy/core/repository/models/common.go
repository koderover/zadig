package models

type SubjectKind string

const (
	MethodAll                = "*"
	KindResource             = "resource"
	UserKind     SubjectKind = "user"
	GroupKind    SubjectKind = "group"
)

// Rule holds information that describes a policy rule, but does not contain information
// about whom the rule applies to.
// If Kind is "resource", verbs are resource actions, while resources are resource names
type Rule struct {
	// Verbs is a list of http methods or resource actions that apply to ALL the Resources contained in this rule. '*' represents all methods.
	Verbs []string `bson:"verbs"         json:"verbs"`

	// Resources is a list of resources this rule applies to. '*' represents all resources.
	Resources       []string         `bson:"resources" json:"resources"`
	Kind            string           `bson:"kind"     json:"kind"`
	MatchAttributes []MatchAttribute `bson:"match_attributes" json:"match_attributes"`
}

type MatchAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Subject contains a reference to the object or user identities a role binding applies to.
type Subject struct {
	// Kind of object being referenced. allowed values are "User", "Group".
	Kind SubjectKind `bson:"kind" json:"kind"`
	// unique identifier of the object being referenced.
	UID string `bson:"uid" json:"uid"`
}
