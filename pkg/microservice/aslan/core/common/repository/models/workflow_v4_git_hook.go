package models

import (
	"github.com/koderover/zadig/v2/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WorkflowV4GitHook struct {
	ID                  primitive.ObjectID  `bson:"_id,omitempty"       yaml:"-"                   json:"id"`
	Name                string              `bson:"name"                      json:"name"`
	WorkflowName        string              `bson:"workflow_name"             json:"workflow_name"`
	ProjectName         string              `bson:"project_name"              json:"project_name"`
	AutoCancel          bool                `bson:"auto_cancel"               json:"auto_cancel"`
	CheckPatchSetChange bool                `bson:"check_patch_set_change"    json:"check_patch_set_change"`
	Enabled             bool                `bson:"enabled"                   json:"enabled"`
	MainRepo            *MainHookRepo       `bson:"main_repo"                 json:"main_repo"`
	Description         string              `bson:"description,omitempty"     json:"description,omitempty"`
	Repos               []*types.Repository `bson:"-"                         json:"repos,omitempty"`
	IsManual            bool                `bson:"is_manual"                 json:"is_manual"`
	WorkflowArg         *WorkflowV4         `bson:"workflow_arg"              json:"workflow_arg"`
}

func (WorkflowV4GitHook) TableName() string {
	return "workflow_v4_git_hook"
}
