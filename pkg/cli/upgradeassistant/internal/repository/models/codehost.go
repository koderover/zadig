package models

type CodeHost struct {
	ID          int    `bson:"id"           json:"id"`
	Type        string `bson:"type"         json:"type"`
	Address     string `bson:"address"      json:"address"`
	Namespace   string `bson:"namespace"    json:"namespace"`
	AccessToken string `bson:"access_token" json:"accessToken"`
}

func (CodeHost) TableName() string {
	return "code_host"
}
