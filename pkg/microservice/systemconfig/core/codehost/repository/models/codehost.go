package models

type CodeHost struct {
	ID             int    `bson:"id"`
	Name           string `bson:"name"             json:"name"`
	Type           string `bson:"type"             json:"type"`
	Address        string `bson:"address"          json:"address"`
	IsReadyV2      bool   `bson:"is_ready_v2"      json:"is_ready_v2"`
	AccessToken    string `bson:"access_token"     json:"accessToken"`
	RefreshToken   string `bson:"refresh_token"    json:"refreshToken"`
	Namespace      string `bson:"namespace"        json:"namespace"`
	OrganizationID int    `bson:"organization_id"  json:"orgId"`
	ApplicationId  string `bson:"application_id"   json:"applicationId"`
	Region         string `bson:"region"           json:"region,omitempty"`
	Username       string `bson:"username"         json:"username,omitempty"`
	Password       string `bson:"password"         json:"password,omitempty"`
	ClientSecret   string `bson:"client_secret"    json:"clientSecret"`
	CreatedAt      int64  `bson:"created_at"       json:"created_at"`
	UpdatedAt      int64  `bson:"updated_at"       json:"updated_at"`
}

func (CodeHost) TableName() string {
	return "code_host"
}
