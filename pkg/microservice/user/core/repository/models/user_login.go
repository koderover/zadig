package models

type UserLogin struct {
	Model
	UID           string `json:"uid"`
	Password      string `json:"password"`
	LastLoginTime int64  `json:"last_login_time"`
	LoginId       string `json:"login_id"`
	LoginType     int    `json:"login_type"`
}

// TableName sets the insert table name for this struct type
func (UserLogin) TableName() string {
	return "user_login"
}
