package models

type Feature struct {
	Name    string `bson:"name"          json:"name"`
	Enabled bool   `bson:"enabled"       json:"enabled"`
}

func (f *Feature) TableName() string {
	return "feature"
}
