package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProjectClusterRelation struct {
	ID          primitive.ObjectID `json:"id,omitempty"              bson:"_id,omitempty"`
	ProjectName string             `json:"project_name"              bson:"project_name"`
	ClusterID   string             `json:"cluster_id"                bson:"cluster_id"`
	CreatedAt   int64              `json:"createdAt"                 bson:"createdAt"`
	CreatedBy   string             `json:"createdBy"                 bson:"createdBy"`
}

func (ProjectClusterRelation) TableName() string {
	return "project_cluster_relation"
}
