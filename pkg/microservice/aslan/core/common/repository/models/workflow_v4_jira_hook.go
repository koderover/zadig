package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type WorkflowV4JiraHook struct {
	ID                       primitive.ObjectID `bson:"_id,omitempty"       yaml:"-"                   json:"id"`
	Name                     string             `bson:"name" json:"name"`
	WorkflowName             string             `bson:"workflow_name"             json:"workflow_name"`
	ProjectName              string             `bson:"project_name"              json:"project_name"`
	Enabled                  bool               `bson:"enabled" json:"enabled"`
	Description              string             `bson:"description" json:"description"`
	JiraID                   string             `bson:"jira_id" json:"jira_id"`
	JiraSystemIdentity       string             `bson:"jira_system_identity" json:"jira_system_identity"`
	JiraURL                  string             `bson:"jira_url"             json:"jira_url"`
	EnabledIssueStatusChange bool               `bson:"enabled_issue_status_change" json:"enabled_issue_status_change"`
	FromStatus               JiraHookStatus     `bson:"from_status" json:"from_status"`
	ToStatus                 JiraHookStatus     `bson:"to_status" json:"to_status"`
	WorkflowArg              *WorkflowV4        `bson:"workflow_arg" json:"workflow_arg"`
}

func (WorkflowV4JiraHook) TableName() string {
	return "workflow_v4_jira_hook"
}
