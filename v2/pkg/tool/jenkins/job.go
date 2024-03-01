package jenkins

import "github.com/koderover/zadig/v2/pkg/microservice/aslan/config"

type ParameterType string

var (
	Choice ParameterType = "ChoiceParameterDefinition"
	Bool   ParameterType = "BooleanParameterDefinition"
	Text   ParameterType = "TextParameterDefinition"
	String ParameterType = "StringParameterDefinition"
)

var ParameterTypeMap = map[ParameterType]config.ParamType{
	Choice: config.ParamTypeChoice,
	Bool:   config.ParamTypeBool,
	Text:   config.ParamTypeString,
	String: config.ParamTypeString,
}

type ListJobsResp struct {
	Class string `json:"_class"`
	Jobs  []Jobs `json:"jobs"`
}
type Jobs struct {
	Class string `json:"_class"`
	Name  string `json:"name"`
}

func (c *Client) ListJob() (resp *ListJobsResp, err error) {
	_, err = c.R().SetSuccessResult(&resp).Get("/api/json?tree=jobs[name]")
	return
}

type Job struct {
	Class                 string        `json:"_class"`
	Actions               []Actions     `json:"actions"`
	Description           string        `json:"description"`
	DisplayName           string        `json:"displayName"`
	DisplayNameOrNull     interface{}   `json:"displayNameOrNull"`
	FullDisplayName       string        `json:"fullDisplayName"`
	FullName              string        `json:"fullName"`
	Name                  string        `json:"name"`
	URL                   string        `json:"url"`
	Buildable             bool          `json:"buildable"`
	Builds                []interface{} `json:"builds"`
	Color                 string        `json:"color"`
	FirstBuild            interface{}   `json:"firstBuild"`
	HealthReport          []interface{} `json:"healthReport"`
	InQueue               bool          `json:"inQueue"`
	KeepDependencies      bool          `json:"keepDependencies"`
	LastBuild             interface{}   `json:"lastBuild"`
	LastCompletedBuild    interface{}   `json:"lastCompletedBuild"`
	LastFailedBuild       interface{}   `json:"lastFailedBuild"`
	LastStableBuild       interface{}   `json:"lastStableBuild"`
	LastSuccessfulBuild   interface{}   `json:"lastSuccessfulBuild"`
	LastUnstableBuild     interface{}   `json:"lastUnstableBuild"`
	LastUnsuccessfulBuild interface{}   `json:"lastUnsuccessfulBuild"`
	NextBuildNumber       int           `json:"nextBuildNumber"`
	Property              []Property    `json:"property"`
	QueueItem             interface{}   `json:"queueItem"`
	ConcurrentBuild       bool          `json:"concurrentBuild"`
	Disabled              bool          `json:"disabled"`
	DownstreamProjects    []interface{} `json:"downstreamProjects"`
	LabelExpression       interface{}   `json:"labelExpression"`
	Scm                   Scm           `json:"scm"`
	UpstreamProjects      []interface{} `json:"upstreamProjects"`
}

func (j Job) GetParameters() []ParameterDefinitions {
	for _, v := range j.Property {
		if v.Class == "hudson.model.ParametersDefinitionProperty" {
			return v.ParameterDefinitions
		}
	}
	return nil
}

type DefaultParameterValue struct {
	Class string      `json:"_class"`
	Value interface{} `json:"value"`
}
type ParameterDefinitions struct {
	Class                 string                `json:"_class"`
	DefaultParameterValue DefaultParameterValue `json:"defaultParameterValue"`
	Description           string                `json:"description"`
	Name                  string                `json:"name"`
	Type                  ParameterType         `json:"type"`
	Choices               []string              `json:"choices,omitempty"`
}
type Actions struct {
	Class                string                 `json:"_class,omitempty"`
	ParameterDefinitions []ParameterDefinitions `json:"parameterDefinitions,omitempty"`
}
type Property struct {
	Class                string                 `json:"_class"`
	ParameterDefinitions []ParameterDefinitions `json:"parameterDefinitions"`
}
type Scm struct {
	Class string `json:"_class"`
}

func (c *Client) GetJob(name string) (resp *Job, err error) {
	_, err = c.R().SetPathParam("job", name).
		SetSuccessResult(&resp).Get("/job/{job}/api/json")
	return
}
