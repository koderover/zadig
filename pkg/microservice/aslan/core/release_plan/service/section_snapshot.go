package service

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"github.com/koderover/zadig/v2/pkg/tool/log"
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
		return buildReleasePlanApprovalSnapshot(plan.Approval)
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
	approvalSnapshot, err := buildReleasePlanApprovalSnapshot(plan.Approval)
	if err != nil {
		return nil, err
	}

	resp := map[string]interface{}{
		"metadata":   buildReleasePlanMetadataSnapshot(plan),
		"approval":   approvalSnapshot,
		"jobs":       make([]interface{}, 0, len(plan.Jobs)),
		"jobs_order": buildReleasePlanJobsOrderSnapshot(plan),
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

func buildReleasePlanApprovalSnapshot(approval *models.Approval) (interface{}, error) {
	if approval == nil {
		return nil, nil
	}

	genericValue, err := toReleasePlanGenericValue(approval)
	if err != nil {
		return nil, err
	}
	return sanitizeReleasePlanValue(filterReleasePlanApprovalInputValue(genericValue)), nil
}

func filterReleasePlanApprovalInputValue(value interface{}) interface{} {
	switch typedValue := value.(type) {
	case map[string]interface{}:
		resp := make(map[string]interface{}, len(typedValue))
		for key, item := range typedValue {
			if shouldDropReleasePlanApprovalInputField(key) {
				continue
			}
			resp[key] = filterReleasePlanApprovalInputValue(item)
		}
		return resp
	case []interface{}:
		resp := make([]interface{}, 0, len(typedValue))
		for _, item := range typedValue {
			resp = append(resp, filterReleasePlanApprovalInputValue(item))
		}
		return resp
	default:
		return value
	}
}

func shouldDropReleasePlanApprovalInputField(key string) bool {
	dropKeys := map[string]struct{}{
		"status":                {},
		"instance_code":         {},
		"instance_id":           {},
		"approval_instance":     {},
		"task_list":             {},
		"timeline":              {},
		"reject_or_approve":     {},
		"operation_time":        {},
		"comment":               {},
		"approval_node_details": {},
	}
	_, exists := dropKeys[key]
	return exists
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
		genericValue, err := toReleasePlanGenericValue(spec)
		if err != nil {
			return nil, err
		}
		specMap, ok := getMapField(genericValue)
		if !ok {
			return nil, nil
		}
		workflowSnapshot, err := buildReleasePlanWorkflowVersionSnapshot(spec, specMap["workflow"])
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"workflow": workflowSnapshot,
		}, nil
	default:
		return sanitizeReleasePlanValue(spec), nil
	}
}

func buildReleasePlanWorkflowVersionSnapshot(spec, rawWorkflow interface{}) (interface{}, error) {
	if workflow, ok := enrichReleasePlanWorkflowWithLatest(spec); ok {
		return buildReleasePlanWorkflowInputSnapshot(workflow)
	}

	return buildReleasePlanWorkflowInputSnapshot(rawWorkflow)
}

func enrichReleasePlanWorkflowWithLatest(spec interface{}) (_ interface{}, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			warnReleasePlanWorkflowRecover(r)
			ok = false
		}
	}()

	workflowSpec := new(models.WorkflowReleaseJobSpec)
	if err := models.IToi(spec, workflowSpec); err != nil || workflowSpec.Workflow == nil {
		return nil, false
	}

	workflowController := controller.CreateWorkflowController(workflowSpec.Workflow)
	if err := workflowController.UpdateWithLatestWorkflow(nil); err != nil || workflowController.WorkflowV4 == nil {
		return nil, false
	}

	return workflowController.WorkflowV4, true
}

func warnReleasePlanWorkflowRecover(recovered interface{}) {
	defer func() {
		_ = recover()
	}()
	log.Warnf("enrich release plan workflow panic: %v", recovered)
}

func buildReleasePlanWorkflowInputSnapshot(workflow interface{}) (interface{}, error) {
	if workflow == nil {
		return nil, nil
	}

	genericValue, err := toReleasePlanGenericValue(workflow)
	if err != nil {
		return nil, err
	}
	workflowMap, ok := getMapField(genericValue)
	if !ok {
		return nil, nil
	}

	resp := make(map[string]interface{})
	for _, key := range []string{
		"id",
		"name",
		"display_name",
		"disabled",
		"category",
		"project",
		"remark",
		"remark_required",
		"ignore_cache",
		"share_storages",
		"concurrency_limit",
	} {
		if value, exists := workflowMap[key]; exists {
			resp[key] = value
		}
	}
	if params, exists := workflowMap["params"]; exists {
		resp["params"] = filterReleasePlanWorkflowInputValue(params)
	}
	if customField, exists := workflowMap["custom_field"]; exists {
		if filtered := filterReleasePlanWorkflowInputValue(customField); filtered != nil {
			resp["custom_field"] = filtered
		}
	}
	if stages, exists := workflowMap["stages"]; exists {
		resp["stages"] = buildReleasePlanWorkflowStagesInputSnapshot(stages)
	}
	if jobs, exists := workflowMap["jobs"]; exists {
		resp["jobs"] = buildReleasePlanWorkflowJobsInputSnapshot(jobs)
	}
	return sanitizeReleasePlanValue(resp), nil
}

func buildReleasePlanWorkflowStagesInputSnapshot(value interface{}) interface{} {
	stages, ok := value.([]interface{})
	if !ok {
		return nil
	}

	resp := make([]interface{}, 0, len(stages))
	for _, stage := range stages {
		stageMap, ok := getMapField(stage)
		if !ok {
			continue
		}
		stageResp := make(map[string]interface{})
		for _, key := range []string{"name", "parallel", "approval", "manual_exec"} {
			if value, exists := stageMap[key]; exists {
				stageResp[key] = filterReleasePlanWorkflowInputValue(value)
			}
		}
		if jobs, exists := stageMap["jobs"]; exists {
			stageResp["jobs"] = buildReleasePlanWorkflowJobsInputSnapshot(jobs)
		}
		if len(stageResp) > 0 {
			resp = append(resp, stageResp)
		}
	}
	return resp
}

func buildReleasePlanWorkflowJobsInputSnapshot(value interface{}) interface{} {
	jobs, ok := value.([]interface{})
	if !ok {
		return nil
	}

	resp := make([]interface{}, 0, len(jobs))
	for _, job := range jobs {
		jobMap, ok := getMapField(job)
		if !ok {
			continue
		}
		jobResp := make(map[string]interface{})
		for _, key := range []string{"name", "type", "run_policy", "error_policy", "execute_policy"} {
			if item, exists := jobMap[key]; exists {
				jobResp[key] = filterReleasePlanWorkflowInputValue(item)
			}
		}
		if serviceModules, exists := jobMap["service_modules"]; exists {
			jobResp["service_modules"] = filterReleasePlanWorkflowInputValue(serviceModules)
		}
		if spec, exists := jobMap["spec"]; exists {
			jobResp["spec"] = filterReleasePlanWorkflowInputValue(spec)
		}
		if len(jobResp) > 0 {
			resp = append(resp, jobResp)
		}
	}
	return resp
}

func filterReleasePlanWorkflowInputValue(value interface{}) interface{} {
	switch typedValue := value.(type) {
	case map[string]interface{}:
		resp := make(map[string]interface{}, len(typedValue))
		for key, item := range typedValue {
			if key == "plugin" {
				filteredPlugin := filterReleasePlanPluginTemplateInputValue(item)
				if filteredPlugin != nil {
					resp[key] = filteredPlugin
				}
				continue
			}
			if shouldDropReleasePlanWorkflowInputField(key) {
				continue
			}
			resp[key] = filterReleasePlanWorkflowInputValue(item)
		}
		return resp
	case []interface{}:
		resp := make([]interface{}, 0, len(typedValue))
		for _, item := range typedValue {
			resp = append(resp, filterReleasePlanWorkflowInputValue(item))
		}
		return resp
	default:
		return value
	}
}

func filterReleasePlanPluginTemplateInputValue(value interface{}) interface{} {
	plugin, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}

	inputs, exists := plugin["inputs"]
	if !exists {
		return nil
	}

	return map[string]interface{}{
		"inputs": filterReleasePlanWorkflowInputValue(inputs),
	}
}

func shouldDropReleasePlanWorkflowInputField(key string) bool {
	if key == "" {
		return false
	}

	dropKeys := map[string]struct{}{
		"last_status":         {},
		"updated":             {},
		"executed_by":         {},
		"executed_time":       {},
		"hook_payload":        {},
		"hash":                {},
		"notification_id":     {},
		"created_by":          {},
		"create_time":         {},
		"updated_by":          {},
		"update_time":         {},
		"approval_instance":   {},
		"operation_time":      {},
		"reject_or_approve":   {},
		"manual_exector_id":   {},
		"manual_exector_name": {},
		"notification_sent":   {},
		"advanced_setting":    {},
		"runtime":             {},
		"steps":               {},
		"properties":          {},
		"outputs":             {},
	}
	if _, exists := dropKeys[key]; exists {
		return true
	}

	return false
}

func releasePlanVersionDiffGroup(sectionKey, sectionName string) (string, string, string) {
	return sectionKey, releasePlanVersionSectionName(sectionKey, sectionName), releasePlanVersionSectionGroupType(sectionKey)
}
