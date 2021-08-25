package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type OperationLog struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"               json:"id,omitempty"`
	Username       string             `bson:"username"                    json:"username"`
	ProductName    string             `bson:"product_name"                json:"product_name"`
	Method         string             `bson:"method"                      json:"method"`
	PermissionUUID string             `bson:"permission_uuid"             json:"permission_uuid"`
	Function       string             `bson:"function"                    json:"function"`
	Name           string             `bson:"name"                        json:"name"`
	RequestBody    string             `bson:"request_body"                json:"request_body"`
	Status         int                `bson:"status"                      json:"status"`
	CreatedAt      int64              `bson:"created_at"                  json:"created_at"`
}

func (OperationLog) TableName() string {
	return "operation_log"
}
