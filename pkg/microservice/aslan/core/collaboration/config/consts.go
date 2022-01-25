package config

type CollaborationType string

const (
	CollaborationShare CollaborationType = "share"
	CollaborationNew   CollaborationType = "new"
)

type DeployType string

const (
	K8sDeploy  DeployType = "k8s"
	HelmDeploy DeployType = "helm"
)
