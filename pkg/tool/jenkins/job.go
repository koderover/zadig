package jenkins

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
)

const listJobsTree = "jobs[name,fullName,_class]"

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
	Class    string `json:"_class"`
	Name     string `json:"name"`
	FullName string `json:"fullName"`
}

func (c *Client) ListJob() (resp *ListJobsResp, err error) {
	_, err = c.R().SetSuccessResult(&resp).Get("/api/json?tree=jobs[name]")
	return
}

func (c *Client) ListJobNames() ([]string, error) {
	jobNames, err := c.listJobNames("")
	if err != nil {
		return nil, err
	}
	sort.Strings(jobNames)

	return jobNames, nil
}

func (c *Client) listJobNames(parent string) ([]string, error) {
	resp := new(ListJobsResp)
	jobPath := JobPath(parent)
	if parent != "" && jobPath == "" {
		return nil, fmt.Errorf("jenkins job name is empty")
	}

	urlPath := jobPath + "/api/json?tree=" + listJobsTree
	_, err := c.R().SetSuccessResult(resp).Get(urlPath)
	if err != nil {
		return nil, err
	}

	jobNames := make([]string, 0)
	for _, job := range resp.Jobs {
		fullName := job.FullName
		if fullName == "" {
			if parent == "" {
				fullName = job.Name
			} else {
				fullName = parent + "/" + job.Name
			}
		}

		if isFolderLikeJob(job.Class) {
			childJobNames, err := c.listJobNames(fullName)
			if err != nil {
				return nil, err
			}
			jobNames = append(jobNames, childJobNames...)
			continue
		}

		jobNames = append(jobNames, fullName)
	}
	return jobNames, nil
}

func isFolderLikeJob(class string) bool {
	return strings.Contains(class, ".folder.") ||
		strings.Contains(class, "Folder") ||
		strings.Contains(class, "MultiBranchProject")
}

func JobPath(name string) string {
	parts := strings.Split(strings.Trim(name, "/"), "/")
	escapedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		escapedParts = append(escapedParts, url.PathEscape(part))
	}
	if len(escapedParts) == 0 {
		return ""
	}

	return "/job/" + strings.Join(escapedParts, "/job/")
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
	jobPath := JobPath(name)
	if jobPath == "" {
		return nil, fmt.Errorf("jenkins job name is empty")
	}

	_, err = c.R().SetSuccessResult(&resp).Get(jobPath + "/api/json")
	return
}
