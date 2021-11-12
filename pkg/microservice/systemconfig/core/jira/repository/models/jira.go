package models

type Jira struct {
	Host        string `bson:"host"         json:"host"`
	User        string `bson:"user"         json:"user"`
	AccessToken string `bson:"access_token" json:"access_token"`
	CreatedAt   int64  `bson:"created_at"   json:"created_at"`
	UpdatedAt   int64  `bson:"updated_at"   json:"updated_at"`
	DeletedAt   int64  `bson:"deleted_at"   json:"deleted_at"`
}

func (Jira) TableName() string {
	return "jira"
}
