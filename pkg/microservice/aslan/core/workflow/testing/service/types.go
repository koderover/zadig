/*
Copyright 2022 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"fmt"
	"strings"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

const DefaultScanningTimeout = 60 * 60

type Scanning struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	ProjectName    string               `json:"project_name"`
	Description    string               `json:"description"`
	ScannerType    types.ScannerType    `json:"scanner_type"`
	EnableScanner  bool                 `json:"enable_scanner"`
	ImageID        string               `json:"image_id"`
	SonarID        string               `json:"sonar_id"`
	Infrastructure string               `json:"infrastructure"`
	VMLabels       []string             `json:"vm_labels"`
	Installs       []*commonmodels.Item `json:"installs"`
	Repos          []*types.Repository  `json:"repos"`
	// Parameter is for sonarQube type only
	Parameter string `json:"parameter"`
	// Envs is the user defined key/values
	Envs             []*commonmodels.KeyVal                `json:"envs"`
	ScriptType       types.ScriptType                      `json:"script_type"`
	Script           string                                `json:"script"`
	AdvancedSetting  *commonmodels.ScanningAdvancedSetting `json:"advanced_settings"`
	CheckQualityGate bool                                  `json:"check_quality_gate"`
	Outputs          []*commonmodels.Output                `json:"outputs"`
	NotifyCtls       []*commonmodels.NotifyCtl             `json:"notify_ctls"`

	// Code Review Configs
	ReviewIncludePaths []string                   `json:"review_include_paths"`
	ReviewExcludePaths []string                   `json:"review_exclude_paths"`
	ReviewFailOn       []string                   `json:"review_fail_on"`
	ReviewRules        []*commonmodels.ReviewRule `json:"review_rules"`

	// template IDs
	TemplateID string `json:"template_id"`
}

// TODO: change the logic of create scanning
type OpenAPICreateScanningReq struct {
	Name        string                    `json:"name"`
	ProjectName string                    `json:"project_key"`
	Description string                    `json:"description"`
	ScannerType types.ScannerType         `json:"scanner_type"`
	ImageName   string                    `json:"image_name"`
	RepoInfo    []*types.OpenAPIRepoInput `json:"repo_info"`
	SonarSystem string                    `json:"sonar_system"`
	// FIMXE: currently only one sonar system is required, so we just fill in the default sonar ID.
	Addons            []*commonmodels.Item          `json:"addons"`
	PrelaunchScript   string                        `json:"prelaunch_script"`
	SonarParameter    string                        `json:"sonar_parameter"`
	Script            string                        `json:"script"`
	EnableQualityGate bool                          `json:"enable_quality_gate"`
	AdvancedSetting   *types.OpenAPIAdvancedSetting `json:"advanced_settings"`
}

type OpenAPICreateScanningTaskReq struct {
	ProjectName string
	ScanName    string
	ScanRepos   []*ScanningRepoInfo    `json:"scan_repos"`
	ScanKVs     []*commonmodels.KeyVal `json:"scan_kvs"`
}

func (s *OpenAPICreateScanningTaskReq) Validate() (bool, error) {
	if s.ProjectName == "" {
		return false, fmt.Errorf("project key cannot be empty")
	}
	if s.ScanName == "" {
		return false, fmt.Errorf("scan name cannot be empty")
	}
	for _, repo := range s.ScanRepos {
		if repo.Branch == "" {
			return false, fmt.Errorf("branch cannot be empty")
		}
	}

	return true, nil
}

type OpenAPICreateScanningTaskResp struct {
	TaskID int64 `json:"task_id"`
}

func (req *OpenAPICreateScanningReq) Validate() (bool, error) {
	if req.Name == "" {
		return false, fmt.Errorf("scanning name cannot be empty")
	}
	if req.ProjectName == "" {
		return false, fmt.Errorf("project key cannot be empty")
	}
	if req.ImageName == "" {
		return false, fmt.Errorf("image name cannot be empty")
	}
	if req.ScannerType != types.ScannerTypeSonarQube && req.ScannerType != types.ScannerTypeOther {
		return false, fmt.Errorf("scanner_type can only be sonarQube or other")
	}

	return true, nil
}

type ListScanningRespItem struct {
	ID          string                 `json:"id"`
	Type        types.ScannerType      `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Statistics  *ScanningStatistic     `json:"statistics"`
	CreatedAt   int64                  `json:"created_at"`
	UpdatedAt   int64                  `json:"updated_at"`
	Repos       []*types.Repository    `json:"repos"`
	ClusterID   string                 `json:"cluster_id"`
	Envs        []*commonmodels.KeyVal `json:"key_vals"`
}

type CreateScanningTaskReq struct {
	KeyVals     commonmodels.KeyValList   `json:"key_vals"`
	Repos       []*ScanningRepoInfo       `json:"repos"`
	HookPayload *commonmodels.HookPayload `json:"hook_payload"`
}

type ScanningRepoInfo struct {
	CodehostID    int      `json:"codehost_id"`
	Source        string   `json:"source"`
	RepoOwner     string   `json:"repo_owner"`
	RepoNamespace string   `json:"repo_namespace"`
	RepoName      string   `json:"repo_name"`
	PR            int      `json:"pr"`
	PRs           []int    `json:"prs"`
	Branch        string   `json:"branch"`
	MergeBranches []string `json:"merge_branches"`
	Tag           string   `json:"tag"`
	DepotType     string   `json:"depot_type"`
	Stream        string   `json:"stream"`
	ViewMapping   string   `json:"view_mapping"`
	ChangeListID  int      `json:"changelist_id"`
	ShelveID      int      `json:"shelve_id"`
}

func (repo *ScanningRepoInfo) GetRepoNamespace() string {
	if repo.RepoNamespace != "" {
		return repo.RepoNamespace
	}
	return repo.RepoOwner
}

func (repo *ScanningRepoInfo) GetKey() string {
	return strings.Join([]string{repo.Source, repo.GetRepoNamespace(), repo.RepoName}, "/")
}

func (repo *ScanningRepoInfo) NormalizeSinglePR() error {
	if repo == nil {
		return fmt.Errorf("repository cannot be nil")
	}
	if len(repo.PRs) > 1 {
		return fmt.Errorf("multiple pull or merge requests are not supported")
	}
	if len(repo.PRs) == 0 {
		if repo.PR <= 0 {
			return fmt.Errorf("pull or merge request ID is required")
		}
		repo.PRs = []int{repo.PR}
		return nil
	}
	if repo.PRs[0] <= 0 {
		return fmt.Errorf("pull or merge request ID must be greater than zero")
	}
	if repo.PR > 0 && repo.PR != repo.PRs[0] {
		return fmt.Errorf("pull or merge request IDs conflict: pr=%d, prs=%v", repo.PR, repo.PRs)
	}
	repo.PR = repo.PRs[0]
	return nil
}

type ScanningStatistic struct {
	TimesRun       int64 `json:"times_run"`
	AverageRuntime int64 `json:"run_time_average"`
}

type ListScanningTaskResp struct {
	ScanInfo   *ScanningInfo       `json:"scan_info"`
	ScanTasks  []*ScanningTaskResp `json:"scan_tasks"`
	TotalTasks int64               `json:"total_tasks"`
}

type ScanningInfo struct {
	Editor    string `json:"editor"`
	UpdatedAt int64  `json:"updated_at"`
}

type ScanningTaskResp struct {
	ScanID    int64               `json:"scan_id"`
	Status    string              `json:"status"`
	RunTime   int64               `json:"run_time"`
	Creator   string              `json:"creator"`
	CreatedAt int64               `json:"created_at"`
	RepoInfo  []*types.Repository `json:"repo_info"`
}

type ScanningTaskDetail struct {
	Creator        string                `json:"creator"`
	Status         string                `json:"status"`
	Error          string                `json:"error,omitempty"`
	CreateTime     int64                 `json:"create_time"`
	EndTime        int64                 `json:"end_time"`
	RepoInfo       []*types.Repository   `json:"repo_info"`
	SonarMetrics   *step.SonarMetrics    `json:"sonar_metrics"`
	ResultLink     string                `json:"result_link,omitempty"`
	IsHasArtifact  bool                  `json:"is_has_artifact"`
	JobName        string                `json:"job_name"`
	JobDisplayName string                `json:"job_display_name"`
	AIReviewReport *AIReviewReportResult `json:"ai_review_report,omitempty"`
}

type AIReviewReportResult struct {
	Report          *step.AIReviewReport `json:"report,omitempty"`
	CollectionError string               `json:"collection_error,omitempty"`
}

func ConvertToDBScanningModule(args *Scanning) *commonmodels.Scanning {
	// ID is omitted since they are of different type and there will be no use of it
	return &commonmodels.Scanning{
		Name:               args.Name,
		ProjectName:        args.ProjectName,
		Description:        args.Description,
		ScannerType:        args.ScannerType,
		EnableScanner:      args.EnableScanner,
		ImageID:            args.ImageID,
		Infrastructure:     args.Infrastructure,
		VMLabels:           args.VMLabels,
		SonarID:            args.SonarID,
		Repos:              args.Repos,
		Parameter:          args.Parameter,
		ScriptType:         args.ScriptType,
		Script:             args.Script,
		AdvancedSetting:    args.AdvancedSetting,
		Installs:           args.Installs,
		CheckQualityGate:   args.CheckQualityGate,
		Outputs:            args.Outputs,
		Envs:               args.Envs,
		TemplateID:         args.TemplateID,
		ReviewIncludePaths: args.ReviewIncludePaths,
		ReviewExcludePaths: args.ReviewExcludePaths,
		ReviewFailOn:       args.ReviewFailOn,
		ReviewRules:        args.ReviewRules,
	}
}

func ConvertDBScanningModule(scanning *commonmodels.Scanning) *Scanning {
	for _, repo := range scanning.Repos {
		repo.RepoNamespace = repo.GetRepoNamespace()
	}
	return &Scanning{
		ID:                 scanning.ID.Hex(),
		Name:               scanning.Name,
		ProjectName:        scanning.ProjectName,
		Description:        scanning.Description,
		ScannerType:        scanning.ScannerType,
		EnableScanner:      scanning.EnableScanner,
		ImageID:            scanning.ImageID,
		SonarID:            scanning.SonarID,
		Infrastructure:     scanning.Infrastructure,
		VMLabels:           scanning.VMLabels,
		Repos:              scanning.Repos,
		Parameter:          scanning.Parameter,
		ScriptType:         scanning.ScriptType,
		Script:             scanning.Script,
		AdvancedSetting:    scanning.AdvancedSetting,
		Installs:           scanning.Installs,
		CheckQualityGate:   scanning.CheckQualityGate,
		Outputs:            scanning.Outputs,
		Envs:               scanning.Envs,
		TemplateID:         scanning.TemplateID,
		ReviewIncludePaths: scanning.ReviewIncludePaths,
		ReviewExcludePaths: scanning.ReviewExcludePaths,
		ReviewFailOn:       scanning.ReviewFailOn,
		ReviewRules:        scanning.ReviewRules,
	}
}

type OpenAPICreateTestTaskReq struct {
	ProjectName string                    `json:"project_key"`
	TestName    string                    `json:"test_name"`
	RepoInfo    []*types.OpenAPIRepoInput `json:"repo_info,omitempty"`
	Inputs      []*types.KV               `json:"inputs,omitempty"`
}

func (t *OpenAPICreateTestTaskReq) Validate() (bool, error) {
	if t.ProjectName == "" {
		return false, fmt.Errorf("project key cannot be empty")
	}
	if t.TestName == "" {
		return false, fmt.Errorf("test name cannot be empty")
	}
	repositories := make(map[string]struct{}, len(t.RepoInfo))
	for i, repo := range t.RepoInfo {
		if repo == nil {
			return false, fmt.Errorf("repo_info[%d] cannot be empty", i)
		}
		if strings.TrimSpace(repo.CodeHostName) == "" || strings.TrimSpace(repo.RepoNamespace) == "" || strings.TrimSpace(repo.RepoName) == "" {
			return false, fmt.Errorf("repo_info[%d] codehost_name, repo_namespace and repo_name cannot be empty", i)
		}
		if repo.EnableCommit {
			if strings.TrimSpace(repo.CommitID) == "" {
				return false, fmt.Errorf("repo_info[%d] commit_id cannot be empty when enable_commit is true", i)
			}
		} else if strings.TrimSpace(repo.Branch) == "" {
			return false, fmt.Errorf("repo_info[%d] branch cannot be empty when enable_commit is false", i)
		}
		repository := strings.TrimSpace(repo.CodeHostName) + "\n" + strings.TrimSpace(repo.RepoNamespace) + "\n" + strings.TrimSpace(repo.RepoName)
		if _, ok := repositories[repository]; ok {
			return false, fmt.Errorf("repo_info[%d] duplicates repository %s/%s", i, repo.RepoNamespace, repo.RepoName)
		}
		repositories[repository] = struct{}{}
	}
	inputs := make(map[string]struct{}, len(t.Inputs))
	for i, input := range t.Inputs {
		if input == nil || strings.TrimSpace(input.Key) == "" {
			return false, fmt.Errorf("inputs[%d] key cannot be empty", i)
		}
		key := strings.TrimSpace(input.Key)
		if _, ok := inputs[key]; ok {
			return false, fmt.Errorf("inputs[%d] duplicates key %q", i, input.Key)
		}
		inputs[key] = struct{}{}
	}

	return true, nil
}

type OpenAPICreateTestTaskResp struct {
	TaskID int64 `json:"task_id"`
}

type OpenAPITestInfo struct {
	ProjectKey string              `json:"project_key"`
	TestName   string              `json:"test_name"`
	Inputs     []*OpenAPITestInput `json:"inputs"`
}

type OpenAPITestInput struct {
	Key          string   `json:"key"`
	Value        string   `json:"value,omitempty"`
	Type         string   `json:"type,omitempty"`
	ChoiceOption []string `json:"choice_option,omitempty"`
	Required     bool     `json:"required"`
	IsCredential bool     `json:"is_credential"`
	HasValue     bool     `json:"has_value"`
	Description  string   `json:"description,omitempty"`
}

type OpenAPIScanTaskDetail struct {
	ScanName   string                  `json:"scan_name"`
	Creator    string                  `json:"creator"`
	TaskID     int64                   `json:"task_id"`
	Status     string                  `json:"status"`
	CreateTime int64                   `json:"create_time"`
	EndTime    int64                   `json:"end_time"`
	ResultLink string                  `json:"result_link"`
	RepoInfo   []*OpenAPIScanRepoBrief `json:"repo_info"`
}

type OpenAPIScanRepoBrief struct {
	RepoOwner    string `json:"repo_owner"`
	Source       string `json:"source"`
	Address      string `json:"address"`
	Branch       string `json:"branch"`
	RemoteName   string `json:"remote_name"`
	RepoName     string `json:"repo_name"`
	Hidden       bool   `json:"hidden"`
	CheckoutPath string `json:"checkout_path"`
	SubModules   bool   `json:"submodules"`
}

type OpenAPITestTaskDetail struct {
	TestName   string             `json:"test_name"`
	TaskID     int64              `json:"task_id"`
	Creator    string             `json:"creator"`
	CreateTime int64              `json:"create_time"`
	StartTime  int64              `json:"start_time"`
	EndTime    int64              `json:"end_time"`
	Status     string             `json:"status"`
	TestReport *OpenAPITestReport `json:"test_report"`
}

type OpenAPITestReport struct {
	TestTotal    int                `json:"test_total"`
	FailureTotal int                `json:"failure_total"`
	SuccessTotal int                `json:"success_total"`
	SkipedTotal  int                `json:"skiped_total"`
	ErrorTotal   int                `json:"error_total"`
	Time         float64            `json:"time"`
	TestCases    []*OpenAPITestCase `json:"test_cases"`
}

type OpenAPITestCase struct {
	Name    string                `json:"name"`
	Time    float64               `json:"time"`
	Failure *commonmodels.Failure `json:"failure"`
	Error   *commonmodels.Error   `json:"error"`
}

const (
	// test
	VerbGetTest    = "get_test"
	VerbCreateTest = "create_test"
	VerbEditTest   = "edit_test"
	VerbDeleteTest = "delete_test"
	VerbRunTest    = "run_test"
)
