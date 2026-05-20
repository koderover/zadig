package service

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

const (
	releasePlanVersionSectionPlan      = "plan"
	releasePlanVersionSectionMetadata  = "metadata"
	releasePlanVersionSectionApproval  = "approval"
	releasePlanVersionSectionJobsOrder = "jobs_order"
	releasePlanVersionSectionJobPrefix = "job:"
)

func releasePlanVersionSectionName(sectionKey, fallbackName string) string {
	switch {
	case sectionKey == releasePlanVersionSectionPlan:
		return "发布计划"
	case sectionKey == releasePlanVersionSectionMetadata:
		return "基础信息"
	case sectionKey == releasePlanVersionSectionApproval:
		return "审批配置"
	case sectionKey == releasePlanVersionSectionJobsOrder:
		return "发布内容顺序"
	case strings.HasPrefix(sectionKey, releasePlanVersionSectionJobPrefix):
		if fallbackName != "" {
			return fallbackName
		}
		return "发布内容"
	default:
		return fallbackName
	}
}

func releasePlanVersionSectionGroupType(sectionKey string) string {
	switch {
	case sectionKey == releasePlanVersionSectionMetadata:
		return "metadata"
	case sectionKey == releasePlanVersionSectionApproval:
		return "approval"
	case sectionKey == releasePlanVersionSectionJobsOrder:
		return "jobs_order"
	case strings.HasPrefix(sectionKey, releasePlanVersionSectionJobPrefix):
		return "job"
	default:
		return "plan"
	}
}

func cloneReleasePlan(plan *models.ReleasePlan) (*models.ReleasePlan, error) {
	if plan == nil {
		return nil, errors.New("nil release plan")
	}

	payload, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}

	resp := new(models.ReleasePlan)
	if err := json.Unmarshal(payload, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func releasePlanVersionSectionKeyByVerb(planBefore, planAfter *models.ReleasePlan, args *UpdateReleasePlanArgs) (string, string, error) {
	if args == nil {
		return releasePlanVersionSectionPlan, "发布计划", nil
	}

	switch args.Verb {
	case VerbUpdateName, VerbUpdateDesc, VerbUpdateTimeRange, VerbUpdateScheduleExecuteTime, VerbUpdateManager, VerbUpdateJiraSprint:
		return releasePlanVersionSectionMetadata, "基础信息", nil
	case VerbUpdateApproval, VerbDeleteApproval:
		return releasePlanVersionSectionApproval, "审批配置", nil
	case VerbReorderReleaseJob:
		return releasePlanVersionSectionJobsOrder, "发布内容顺序", nil
	case VerbUpdateReleaseJob, VerbDeleteReleaseJob:
		jobID, _ := extractReleasePlanJobID(args.Spec)
		if jobID == "" {
			return "", "", errors.New("missing release job id")
		}
		jobName := releasePlanVersionSectionJobName(planAfter, jobID)
		if jobName == "" {
			jobName = releasePlanVersionSectionJobName(planBefore, jobID)
		}
		return releasePlanVersionSectionJobPrefix + jobID, jobName, nil
	case VerbCreateReleaseJob:
		createdJob := findCreatedReleasePlanJob(planBefore, planAfter)
		if createdJob == nil {
			return "", "", errors.New("failed to locate created release job")
		}
		return releasePlanVersionSectionJobPrefix + createdJob.ID, createdJob.Name, nil
	default:
		return releasePlanVersionSectionPlan, "发布计划", nil
	}
}

func extractReleasePlanJobID(spec interface{}) (string, error) {
	if spec == nil {
		return "", nil
	}
	payload, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	resp := struct {
		ID string `json:"id"`
	}{}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func releasePlanVersionSectionJobName(plan *models.ReleasePlan, jobID string) string {
	if plan == nil {
		return ""
	}
	for _, job := range plan.Jobs {
		if job.ID == jobID {
			return job.Name
		}
	}
	return ""
}

func findCreatedReleasePlanJob(planBefore, planAfter *models.ReleasePlan) *models.ReleaseJob {
	if planAfter == nil {
		return nil
	}
	beforeJobIDs := make(map[string]struct{}, len(planBefore.Jobs))
	if planBefore != nil {
		for _, job := range planBefore.Jobs {
			beforeJobIDs[job.ID] = struct{}{}
		}
	}
	for _, job := range planAfter.Jobs {
		if _, exists := beforeJobIDs[job.ID]; !exists {
			return job
		}
	}
	return nil
}

func buildReleasePlanVersionSnapshot(plan *models.ReleasePlan, sectionKey string) (interface{}, error) {
	if plan == nil {
		return nil, nil
	}

	switch {
	case sectionKey == releasePlanVersionSectionPlan:
		return buildReleasePlanInputSnapshot(plan)
	case sectionKey == releasePlanVersionSectionMetadata:
		return buildReleasePlanMetadataSnapshot(plan), nil
	case sectionKey == releasePlanVersionSectionApproval:
		return sanitizeReleasePlanValue(plan.Approval), nil
	case sectionKey == releasePlanVersionSectionJobsOrder:
		return buildReleasePlanJobsOrderSnapshot(plan), nil
	case strings.HasPrefix(sectionKey, releasePlanVersionSectionJobPrefix):
		jobID := strings.TrimPrefix(sectionKey, releasePlanVersionSectionJobPrefix)
		job, err := findReleasePlanJob(plan, jobID)
		if err != nil {
			return nil, nil
		}
		return buildReleasePlanJobInputSnapshot(job)
	default:
		return nil, errors.Errorf("unsupported release plan version section key: %s", sectionKey)
	}
}

func buildReleasePlanInputSnapshot(plan *models.ReleasePlan) (interface{}, error) {
	resp := map[string]interface{}{
		"metadata": buildReleasePlanMetadataSnapshot(plan),
		"approval": sanitizeReleasePlanValue(plan.Approval),
		"jobs":     make([]interface{}, 0, len(plan.Jobs)),
	}
	for _, job := range plan.Jobs {
		snapshot, err := buildReleasePlanJobInputSnapshot(job)
		if err != nil {
			return nil, err
		}
		resp["jobs"] = append(resp["jobs"].([]interface{}), snapshot)
	}
	return resp, nil
}

func buildReleasePlanMetadataSnapshot(plan *models.ReleasePlan) map[string]interface{} {
	if plan == nil {
		return nil
	}
	return map[string]interface{}{
		"name":                    plan.Name,
		"manager":                 plan.Manager,
		"manager_id":              plan.ManagerID,
		"start_time":              plan.StartTime,
		"end_time":                plan.EndTime,
		"schedule_execute_time":   plan.ScheduleExecuteTime,
		"description":             plan.Description,
		"jira_sprint_association": sanitizeReleasePlanValue(plan.JiraSprintAssociation),
	}
}

func buildReleasePlanJobsOrderSnapshot(plan *models.ReleasePlan) []interface{} {
	resp := make([]interface{}, 0)
	if plan == nil {
		return resp
	}
	for _, job := range plan.Jobs {
		resp = append(resp, map[string]interface{}{
			"id":   job.ID,
			"name": job.Name,
		})
	}
	return resp
}

func buildReleasePlanJobInputSnapshot(job *models.ReleaseJob) (interface{}, error) {
	if job == nil {
		return nil, nil
	}

	spec, err := buildReleasePlanJobInputSpec(job.Type, job.Spec)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":         job.ID,
		"name":       job.Name,
		"manager":    job.Manager,
		"manager_id": job.ManagerID,
		"type":       job.Type,
		"spec":       spec,
	}, nil
}

func buildReleasePlanJobInputSpec(jobType config.ReleasePlanJobType, spec interface{}) (interface{}, error) {
	switch jobType {
	case config.JobText:
		inputSpec := new(models.TextReleaseJobSpec)
		if err := models.IToi(spec, inputSpec); err != nil {
			return nil, err
		}
		return sanitizeReleasePlanValue(inputSpec), nil
	case config.JobWorkflow:
		inputSpec := new(models.WorkflowReleaseJobSpec)
		if err := models.IToi(spec, inputSpec); err != nil {
			return nil, err
		}
		return sanitizeReleasePlanValue(map[string]interface{}{
			"workflow": inputSpec.Workflow,
		}), nil
	default:
		return sanitizeReleasePlanValue(spec), nil
	}
}

func releasePlanVersionDiffGroup(sectionKey, sectionName string) (string, string, string) {
	return sectionKey, releasePlanVersionSectionName(sectionKey, sectionName), releasePlanVersionSectionGroupType(sectionKey)
}
