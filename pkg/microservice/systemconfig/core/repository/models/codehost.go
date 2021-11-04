package models

type CodeHost struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Address        string `json:"address"`
	IsReady        bool   `json:"ready"`
	AccessToken    string `json:"accessToken"`
	RefreshToken   string `json:"refreshToken"`
	Namespace      string `json:"namespace"`
	OrganizationID int    `json:"orgId"`
	ApplicationId  string `json:"applicationId"`
	Region         string `json:"region,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	ClientSecret   string `json:"clientSecret"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

func (CodeHost) TableName() string {
	return "code_host"
}
