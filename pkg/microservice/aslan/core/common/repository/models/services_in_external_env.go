package models

type ServicesInExternalEnv struct {
	ProductName string `bson:"product_name"         json:"product_name"`
	ServiceName string `bson:"service_name"         json:"service_name"`
	EnvName     string `bson:"env_name"             json:"env_name"`
	Namespace   string `bson:"namespace"            json:"namespace"`
	ClusterID   string `bson:"cluster_id"           json:"cluster_id"`
}

func (ServicesInExternalEnv) TableName() string {
	return "services_in_external_env"
}
