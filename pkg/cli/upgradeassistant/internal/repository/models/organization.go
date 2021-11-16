package models

type Organization struct {
	ID    int    `bson:"id"              json:"id"`
	Token string `bson:"token,omitempty" json:"token,omitempty"`
}

func (Organization) TableName() string {
	return "organization"
}
