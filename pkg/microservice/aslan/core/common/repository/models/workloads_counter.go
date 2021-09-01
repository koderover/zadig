package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WorkLoadStat struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"         json:"id,omitempty"`
	ClusterID string             `bson:"cluster_id"            json:"cluster_id"`
	Namespace string             `bson:"namespace"             json:"namespace"`
	Workloads []WorkLoad         `bson:"workloads"             json:"workloads"`
}

type WorkLoad struct {
	OccupyBy string `bson:"occupy_by"         json:"occupy_by"`
	Name     string `bson:"name"              json:"name"`
}

func (WorkLoadStat) TableName() string {
	return "workload_stat"
}

type SaveK8sWorkloadsArgs struct {
	WorkLoads []string `bson:"workLoads"         json:"workLoads"`
	EnvName   string   `bson:"env_name"         json:"env_name"`
	ClusterID string   `bson:"cluster_id"         json:"cluster_id"`
	Namespace string   `bson:"namespace"         json:"namespace"`
}
