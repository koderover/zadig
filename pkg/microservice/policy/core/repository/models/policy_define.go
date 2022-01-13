package models

type PolicyType string

const (
	PolicyTypeSystem     PolicyType = "System"
	PolicyTypeUserDefine PolicyType = "UserDefine"
)

type PolicyDefine struct {
	Name            string      `bson:"name"                 json:"name"`
	Namespace       string      `bson:"namespace"            json:"namespace"`
	Alias           string      `bson:"alias"                json:"alias"`
	BindUserIDs     []string    `bson:"bind_user_ids"        json:"bind_user_ids"`
	BindUserGroups  []string    `bson:"bind_user_groups"     json:"bind_user_groups"`
	MatchAttributes []Attribute `bson:"match_attributes"     json:"match_attributes"`
	Rules           []Rule      `bson:"rules"                json:"rules"`
	PolicyType      PolicyType  `bson:"policy_type"          json:"policy_type"`
	CreateTime      int64       `bson:"create_time"          json:"create_time"`
	CreateBy        string      `bson:"create_by"            json:"create_by"`
}

func (PolicyDefine) TableName() string {
	return "policy_define"
}
