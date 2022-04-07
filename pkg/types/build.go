package types

type JenkinsBuildParam struct {
	Name         string           `bson:"name,omitempty"                json:"name,omitempty"`
	Value        interface{}      `bson:"value,omitempty"               json:"value,omitempty"`
	Type         JenkinsParamType `bson:"type,omitempty"                json:"type,omitempty"`
	AutoGenerate bool             `bson:"auto_generate,omitempty"       json:"auto_generate,omitempty"`
	ChoiceOption []string         `bson:"choice_option,omitempty"       json:"choice_option,omitempty"`
}

type JenkinsParamType string

const (
	Str    JenkinsParamType = "string"
	Choice JenkinsParamType = "choice"
)
