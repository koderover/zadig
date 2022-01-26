package models

type PolicyMeta struct {
	Resource    string            `bson:"resource"    json:"resource"`
	Alias       string            `bson:"alias"       json:"alias"`
	Description string            `bson:"description" json:"description"`
	Rules       []*PolicyMetaRule `bson:"rules"       json:"rules"`
}

type PolicyMetaRule struct {
	Action      string        `bson:"action"      json:"action"`
	Alias       string        `bson:"alias"       json:"alias"`
	Description string        `bson:"description" json:"description"`
	Rules       []*ActionRule `bson:"rules"       json:"rules"`
}

type ActionRule struct {
	Method          string      `bson:"method"                     json:"method"`
	Endpoint        string      `bson:"endpoint"                   json:"endpoint"`
	ResourceType    string      `bson:"resource_type,omitempty"    json:"resourceType,omitempty"`
	IDRegex         string      `bson:"id_regex,omitempty"         json:"idRegex,omitempty"`
	MatchAttributes []Attribute `bson:"match_attributes,omitempty" json:"matchAttributes,omitempty"`
}

type Attribute struct {
	Key   string `bson:"key"   json:"key"`
	Value string `bson:"value" json:"value"`
}
