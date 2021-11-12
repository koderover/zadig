package models

type EmailService struct {
	Name        string `json:"name"          bson:"name"`
	Address     string `json:"address"       bson:"address"`
	DisplayName string `json:"display_name"  bson:"display_name"`
	Theme       string `json:"theme"         bson:"theme"`
	CreatedAt   int64  `json:"created_at"    bson:"created_at"`
	UpdatedAt   int64  `json:"updated_at"    bson:"updated_at"`
	DeletedAt   int64  `json:"deleted_at"    bson:"deleted_at"`
}

func (EmailService) TableName() string {
	return "email_service"
}
