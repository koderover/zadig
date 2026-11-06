package models

type EnvironmentInspectionInput struct {
	ProjectKey string `json:"project_key,omitempty"`
	EnvName    string `json:"env_name,omitempty"`
	Production *bool  `json:"production,omitempty"`
}

type EnvironmentHistoryInput struct {
	ProjectKey string `json:"project_key,omitempty"`
	EnvName    string `json:"env_name,omitempty"`
	Production *bool  `json:"production,omitempty"`
	Page       int    `json:"page,omitempty"`
	PageSize   int    `json:"page_size,omitempty"`
}

type EfficiencyDiagnosisInput struct {
	Projects  []string `json:"project_list,omitempty"`
	StartTime int64    `json:"start_time,omitempty"`
	EndTime   int64    `json:"end_time,omitempty"`
	Question  string   `json:"question,omitempty"`
}
