package models

type EmailHost struct {
	Name      string `bson:"name"       json:"name"`
	Port      int    `bson:"port"       json:"port"`
	Username  string `bson:"username"   json:"username"`
	Password  string `bson:"password"   json:"password"`
	IsTLS     bool   `bson:"is_tls"     json:"isTLS"`
	CreatedAt int64  `bson:"created_at" json:"created_at"`
	DeletedAt int64  `bson:"deleted_at" json:"deleted_at"`
	UpdatedAt int64  `bson:"updated_at" json:"updated_at"`
}

func (EmailHost) TableName() string {
	return "email_host"
}
