package models

type CodeHost struct {
	ID   int    `bson:"id"               json:"id"`
	Type string `bson:"type"             json:"type"`
}

func (CodeHost) TableName() string {
	return "code_host"
}
