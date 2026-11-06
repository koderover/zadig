package models

import (
	"encoding/json"
)

type OperationItem struct {
	ID          string          `json:"id"`
	Action      string          `json:"action"`
	ProjectKey  string          `json:"project_key"`
	ProjectType string          `json:"project_type,omitempty"`
	Params      json.RawMessage `json:"params,omitempty"`
	DependsOn   []string        `json:"depends_on,omitempty"`
	Status      string          `json:"status"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
}

type OperationPlan struct {
	RetryOfRunID  string           `json:"retry_of_run_id,omitempty"`
	Items         []OperationItem  `json:"items"`
	Notes         []string         `json:"notes,omitempty"`
	Draft         *OnboardingInput `json:"draft,omitempty"`
	Input         json.RawMessage  `json:"input,omitempty" swaggertype:"object"`
	MissingFields []string         `json:"missing_fields,omitempty"`
}

type RepairInput struct {
	Target          ServiceTarget   `json:"target"`
	Action          string          `json:"action" jsonschema:"enum=restart_service,enum=rollback_environment,description=用户明确请求的修复动作"`
	Revision        int64           `json:"revision,omitempty" jsonschema:"description=回滚目标环境历史版本"`
	PodFingerprint  string          `json:"pod_fingerprint,omitempty"`
	PodNames        []string        `json:"pod_names,omitempty"`
	CurrentRevision int64           `json:"current_revision,omitempty"`
	VersionPreview  json.RawMessage `json:"version_preview,omitempty"`
	HelmChart       bool            `json:"helm_chart" jsonschema:"description=是否以 Helm release 为目标"`
}
