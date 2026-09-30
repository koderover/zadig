/*
Copyright 2026 The KodeRover Authors.

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

package stepcontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/scmnotify"
	statmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	statrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
	jobtypes "github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/job"
	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
)

type aiReviewReportCtl struct {
	step        *commonmodels.StepTask
	reportSpec  *stepspec.StepAIReviewReportSpec
	workflowCtx *commonmodels.WorkflowTaskCtx
	log         *zap.SugaredLogger
}

func NewAIReviewReportCtl(stepTask *commonmodels.StepTask, workflowCtx *commonmodels.WorkflowTaskCtx, log *zap.SugaredLogger) (*aiReviewReportCtl, error) {
	yamlBytes, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal AI review report spec: %w", err)
	}
	reportSpec := new(stepspec.StepAIReviewReportSpec)
	if err := yaml.Unmarshal(yamlBytes, reportSpec); err != nil {
		return nil, fmt.Errorf("unmarshal AI review report spec: %w", err)
	}
	stepTask.Spec = reportSpec
	return &aiReviewReportCtl{step: stepTask, reportSpec: reportSpec, workflowCtx: workflowCtx, log: log}, nil
}

func (s *aiReviewReportCtl) PreRun(ctx context.Context) error {
	return nil
}

func (s *aiReviewReportCtl) AfterRun(ctx context.Context) error {
	if s.workflowCtx.IsDebug {
		return nil
	}
	key := job.GetJobOutputKey(s.step.JobKey, jobtypes.JobAIReviewReportKey)
	reportJSON, ok := s.workflowCtx.GlobalContextGet(key)
	// Consume the output so a retry cannot reuse this attempt's report.
	if s.workflowCtx.GlobalContextSet != nil {
		s.workflowCtx.GlobalContextSet(key, "")
	}
	s.reportSpec.Report = nil
	if !ok {
		s.reportSpec.CollectionError = "AI review report was not returned by the job executor"
		s.step.Spec = s.reportSpec
		return nil
	}
	var report *stepspec.AIReviewReport
	if err := json.Unmarshal([]byte(reportJSON), &report); err != nil {
		s.reportSpec.CollectionError = fmt.Sprintf("AI review report is invalid JSON: %v", err)
		s.step.Spec = s.reportSpec
		s.log.Errorf("decode AI review report: %v", err)
		return nil
	}
	if report == nil {
		s.reportSpec.CollectionError = "AI review report cannot be null"
		s.step.Spec = s.reportSpec
		return nil
	}
	s.reportSpec.Report = report
	s.reportSpec.CollectionError = ""
	s.step.Spec = s.reportSpec
	if s.reportSpec.PR <= 0 {
		return nil
	}
	reviewedAt := time.Now().Unix()
	metadata, err := scmnotify.NewService().PublishAIReviewReport(
		s.workflowCtx.ProjectName,
		s.reportSpec.CodehostID,
		s.reportSpec.RepoOwner,
		s.reportSpec.RepoName,
		s.reportSpec.PR,
		report,
		s.log,
	)
	if err != nil {
		s.log.Warnf("failed to publish AI review result: %v", err)
	}
	s.saveAIReviewStat(metadata, report, reviewedAt)
	return nil
}

func (s *aiReviewReportCtl) saveAIReviewStat(metadata scmnotify.AIReviewPRMetadata, report *stepspec.AIReviewReport, reviewedAt int64) {
	findings := make([]statmodels.AIReviewStatFinding, 0, len(report.Findings))
	for _, finding := range report.Findings {
		findings = append(findings, statmodels.AIReviewStatFinding{
			Fingerprint: finding.Fingerprint, Severity: finding.Severity,
			Category: finding.Category, CategoryName: finding.CategoryName,
			RuleID: finding.RuleID, RuleName: finding.RuleName, MatchedRules: finding.MatchedRules,
		})
	}
	stat := &statmodels.AIReviewStat{
		ProjectName: s.workflowCtx.ProjectName, ProjectDisplayName: s.workflowCtx.ProjectDisplayName,
		WorkflowName: s.workflowCtx.WorkflowName, TaskID: s.workflowCtx.TaskID,
		CodehostID: s.reportSpec.CodehostID, RepoOwner: s.reportSpec.RepoOwner,
		RepoName: s.reportSpec.RepoName, PR: s.reportSpec.PR,
		PRTitle: metadata.Title, PRAuthor: metadata.Author, PRURL: metadata.URL,
		ReviewedAt: reviewedAt, Model: report.Metadata.Model, Usage: report.Usage,
		DurationMS: report.DurationMS, Incomplete: report.Incomplete, ExitCode: report.ExitCode,
		Findings: findings,
	}
	writeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := statrepo.NewAIReviewStatColl().Upsert(writeCtx, stat); err != nil {
		s.log.Errorf("save AI review stat for %s/%s#%d workflow %s task %d: %v", stat.RepoOwner, stat.RepoName, stat.PR, stat.WorkflowName, stat.TaskID, err)
	}
}
