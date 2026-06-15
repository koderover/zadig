package service

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"time"

	cache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonoptions"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"golang.org/x/sync/singleflight"

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

	// Keep this cache window short: it only exists to collapse bursty lookups for the same workflow
	// while avoiding stale data lingering after a real workflow change.
	releasePlanWorkflowLatestSnapshotCacheTTL = 3 * time.Second
	// Use a non-printable separator so cache keys stay unambiguous even if names contain common delimiters.
	releasePlanWorkflowLatestSnapshotCacheKeySeparator = "\x00"
)

var releasePlanWorkflowControllerBSONRegistry = func() *bsoncodec.Registry {
	nilSliceCodec := bsoncodec.NewSliceCodec(bsonoptions.SliceCodec().SetEncodeNilAsEmpty(true))
	tM := reflect.TypeOf(bson.M{})
	return bson.NewRegistryBuilder().RegisterTypeMapEntry(bsontype.EmbeddedDocument, tM).RegisterDefaultEncoder(reflect.Slice, nilSliceCodec).Build()
}()

var (
	releasePlanWorkflowLatestSnapshotGroup         singleflight.Group
	releasePlanWorkflowLatestSnapshotCacheTTLValue = releasePlanWorkflowLatestSnapshotCacheTTL
	releasePlanWorkflowLatestSnapshotCache         = cache.New(releasePlanWorkflowLatestSnapshotCacheTTLValue, releasePlanWorkflowLatestSnapshotCacheTTLValue)
	releasePlanWorkflowLatestSnapshotLoader        = lookupReleasePlanWorkflowLatestSnapshot

	errReleasePlanWorkflowLatestSnapshotUnavailable = errors.New("release plan workflow latest snapshot unavailable")

	releasePlanWorkflowSnapshotTopLevelFields = []string{
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
	}
	releasePlanWorkflowSnapshotStageDisplayFields  = []string{"name", "parallel", "approval", "manual_exec"}
	releasePlanWorkflowSnapshotStageRequiredFields = []string{"parallel", "approval", "manual_exec"}
	releasePlanWorkflowSnapshotJobDisplayFields    = []string{"name", "type", "skipped", "run_policy", "error_policy", "execute_policy"}
	releasePlanWorkflowSnapshotJobRequiredFields   = []string{"run_policy", "error_policy", "execute_policy"}
	// Keep this list minimal. It only gates whether we must pay the cost of loading the latest
	// workflow snapshot, so we require fields that materially affect display completeness.
	// `project` is intentionally optional here for historical compatibility and to avoid
	// unnecessary latest-workflow lookups on older snapshots.
	releasePlanWorkflowSnapshotTopLevelRequiredFields = []string{"name"}
	releasePlanWorkflowLegacyNameKeys                 = []string{"workflowName", "workflow_name"}
	releasePlanWorkflowLegacyProjectKeys              = []string{"projectName", "project_name"}
)

func isReleasePlanVersionMetadataSection(sectionKey string) bool {
	return sectionKey == releasePlanVersionSectionMetadata || strings.HasPrefix(sectionKey, releasePlanVersionSectionMetadata+":")
}

func releasePlanVersionSectionName(sectionKey, fallbackName string) string {
	switch {
	case sectionKey == releasePlanVersionSectionPlan:
		return "发布计划"
	case isReleasePlanVersionMetadataSection(sectionKey):
		if name, exists := releasePlanCollabMetadataSectionNames[sectionKey]; exists {
			return name
		}
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
	case isReleasePlanVersionMetadataSection(sectionKey):
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
	case VerbUpdateName:
		return releasePlanCollabSectionMetadataName, releasePlanVersionSectionName(releasePlanCollabSectionMetadataName, ""), nil
	case VerbUpdateDesc:
		return releasePlanCollabSectionMetadataDescription, releasePlanVersionSectionName(releasePlanCollabSectionMetadataDescription, ""), nil
	case VerbUpdateTimeRange:
		return releasePlanCollabSectionMetadataTimeRange, releasePlanVersionSectionName(releasePlanCollabSectionMetadataTimeRange, ""), nil
	case VerbUpdateScheduleExecuteTime:
		return releasePlanCollabSectionMetadataScheduleExecute, releasePlanVersionSectionName(releasePlanCollabSectionMetadataScheduleExecute, ""), nil
	case VerbUpdateManager:
		return releasePlanCollabSectionMetadataManager, releasePlanVersionSectionName(releasePlanCollabSectionMetadataManager, ""), nil
	case VerbUpdateJiraSprint:
		return releasePlanCollabSectionMetadataJiraSprint, releasePlanVersionSectionName(releasePlanCollabSectionMetadataJiraSprint, ""), nil
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
	case isReleasePlanVersionMetadataSection(sectionKey):
		return buildReleasePlanMetadataSectionSnapshot(plan, sectionKey), nil
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

func buildReleasePlanMetadataSectionSnapshot(plan *models.ReleasePlan, sectionKey string) map[string]interface{} {
	metadata := buildReleasePlanMetadataSnapshot(plan)
	if metadata == nil {
		return nil
	}

	switch sectionKey {
	case releasePlanVersionSectionMetadata:
		return metadata
	case releasePlanCollabSectionMetadataName:
		return map[string]interface{}{
			"name": metadata["name"],
		}
	case releasePlanCollabSectionMetadataManager:
		return map[string]interface{}{
			"manager":    metadata["manager"],
			"manager_id": metadata["manager_id"],
		}
	case releasePlanCollabSectionMetadataTimeRange:
		return map[string]interface{}{
			"start_time": metadata["start_time"],
			"end_time":   metadata["end_time"],
		}
	case releasePlanCollabSectionMetadataScheduleExecute:
		return map[string]interface{}{
			"schedule_execute_time": metadata["schedule_execute_time"],
		}
	case releasePlanCollabSectionMetadataDescription:
		return map[string]interface{}{
			"description": metadata["description"],
		}
	case releasePlanCollabSectionMetadataJiraSprint:
		return map[string]interface{}{
			"jira_sprint_association": metadata["jira_sprint_association"],
		}
	default:
		return metadata
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
		"flat_approve_users":    {},
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
	rawSnapshot, err := buildReleasePlanWorkflowInputSnapshot(rawWorkflow)
	if err != nil {
		return nil, err
	}

	if isCompleteReleasePlanWorkflowSnapshot(rawSnapshot) {
		return rawSnapshot, nil
	}

	if latestSnapshot, ok := loadReleasePlanWorkflowLatestSnapshotWithCache(spec); ok {
		return latestSnapshot, nil
	}

	return rawSnapshot, nil
}

func loadReleasePlanWorkflowLatestSnapshotWithCache(spec interface{}) (_ interface{}, ok bool) {
	cacheKey, cacheable := releasePlanWorkflowLatestSnapshotCacheKey(spec)
	if !cacheable {
		return releasePlanWorkflowLatestSnapshotLoader(spec)
	}

	if snapshot, exists := getReleasePlanWorkflowLatestSnapshotCache(cacheKey); exists {
		return snapshot, true
	}

	// Convert the loader's (snapshot, ok) contract into singleflight's (value, error) contract
	// so all concurrent callers can derive the final bool from the shared error result.
	value, err, _ := releasePlanWorkflowLatestSnapshotGroup.Do(cacheKey, func() (interface{}, error) {
		if snapshot, exists := getReleasePlanWorkflowLatestSnapshotCache(cacheKey); exists {
			return snapshot, nil
		}

		snapshot, ok := releasePlanWorkflowLatestSnapshotLoader(spec)
		if !ok {
			return nil, errReleasePlanWorkflowLatestSnapshotUnavailable
		}

		setReleasePlanWorkflowLatestSnapshotCache(cacheKey, snapshot)
		return snapshot, nil
	})
	if err != nil {
		return nil, false
	}
	return value, true
}

func lookupReleasePlanWorkflowLatestSnapshot(spec interface{}) (_ interface{}, ok bool) {
	workflow, ok := enrichReleasePlanWorkflowWithLatest(spec)
	if !ok {
		return nil, false
	}

	snapshot, err := buildReleasePlanWorkflowInputSnapshot(workflow)
	if err != nil {
		return nil, false
	}
	return snapshot, true
}

func releasePlanWorkflowLatestSnapshotCacheKey(spec interface{}) (string, bool) {
	projectName, workflowName, ok := releasePlanWorkflowLatestSnapshotIdentity(spec)
	if !ok {
		return "", false
	}

	return joinReleasePlanWorkflowLatestSnapshotCacheKey(projectName, workflowName), true
}

func releasePlanWorkflowLatestSnapshotIdentity(spec interface{}) (projectName, workflowName string, ok bool) {
	specMap, ok := getMapField(spec)
	if !ok {
		return "", "", false
	}

	workflowMap := releasePlanWorkflowLatestSnapshotLookupMap(specMap)
	workflowName = firstReleasePlanWorkflowLookupString(workflowMap, "name")
	if workflowName == "" {
		workflowName = firstReleasePlanWorkflowLookupString(specMap, releasePlanWorkflowLegacyNameKeys...)
	}
	if workflowName == "" {
		return "", "", false
	}

	projectName = firstReleasePlanWorkflowLookupString(workflowMap, "project")
	if projectName == "" {
		projectName = firstReleasePlanWorkflowLookupString(specMap, releasePlanWorkflowLegacyProjectKeys...)
	}
	return projectName, workflowName, true
}

func releasePlanWorkflowLatestSnapshotLookupMap(specMap map[string]interface{}) map[string]interface{} {
	workflowMap, _ := getMapField(specMap["workflow"])
	return workflowMap
}

func joinReleasePlanWorkflowLatestSnapshotCacheKey(projectName, workflowName string) string {
	return projectName + releasePlanWorkflowLatestSnapshotCacheKeySeparator + workflowName
}

func getReleasePlanWorkflowLatestSnapshotCache(cacheKey string) (_ interface{}, ok bool) {
	if cacheKey == "" {
		return nil, false
	}

	value, exists := releasePlanWorkflowLatestSnapshotCache.Get(cacheKey)
	if !exists {
		return nil, false
	}
	return value, true
}

func setReleasePlanWorkflowLatestSnapshotCache(cacheKey string, snapshot interface{}) {
	if cacheKey == "" || snapshot == nil {
		return
	}
	releasePlanWorkflowLatestSnapshotCache.Set(cacheKey, snapshot, releasePlanWorkflowLatestSnapshotCacheTTLValue)
}

func resetReleasePlanWorkflowLatestSnapshotCache() {
	releasePlanWorkflowLatestSnapshotCache.Flush()
}

func enrichReleasePlanWorkflowWithLatest(spec interface{}) (_ interface{}, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			warnReleasePlanWorkflowRecover(r)
			ok = false
		}
	}()

	workflowSpec := new(models.WorkflowReleaseJobSpec)
	if err := models.IToi(spec, workflowSpec); err != nil {
		return nil, false
	}
	applyReleasePlanWorkflowLatestLookupCompat(spec, workflowSpec)
	if workflowSpec.Workflow == nil || workflowSpec.Workflow.Name == "" {
		return nil, false
	}
	normalizedWorkflow, err := normalizeReleasePlanWorkflowForController(workflowSpec.Workflow)
	if err != nil {
		return nil, false
	}
	workflowSpec.Workflow = normalizedWorkflow

	workflowController := controller.CreateWorkflowController(workflowSpec.Workflow)
	if err := workflowController.UpdateWithLatestWorkflow(nil); err != nil || workflowController.WorkflowV4 == nil {
		return nil, false
	}

	return workflowController.WorkflowV4, true
}

func isCompleteReleasePlanWorkflowSnapshot(snapshot interface{}) bool {
	workflowMap, ok := getMapField(snapshot)
	if !ok {
		return false
	}

	workflowID, _ := getStringField(workflowMap, "id")
	if workflowID == "" || workflowID == "000000000000000000000000" {
		return false
	}
	if !hasReleasePlanWorkflowSnapshotStringFields(workflowMap, releasePlanWorkflowSnapshotTopLevelRequiredFields...) {
		return false
	}

	if !hasReleasePlanWorkflowSnapshotStageFields(workflowMap["stages"]) {
		return false
	}
	return true
}

func hasReleasePlanWorkflowSnapshotStageFields(value interface{}) bool {
	if value == nil {
		return true
	}

	stages, ok := value.([]interface{})
	if !ok {
		return false
	}
	for _, stage := range stages {
		stageMap, ok := getMapField(stage)
		if !ok {
			return false
		}
		if !hasReleasePlanWorkflowSnapshotFields(stageMap, releasePlanWorkflowSnapshotStageRequiredFields...) {
			return false
		}
		if !hasReleasePlanWorkflowSnapshotJobFields(stageMap["jobs"]) {
			return false
		}
	}
	return true
}

func hasReleasePlanWorkflowSnapshotJobFields(value interface{}) bool {
	jobs, ok := value.([]interface{})
	if !ok {
		return false
	}
	for _, job := range jobs {
		jobMap, ok := getMapField(job)
		if !ok {
			return false
		}
		if !hasReleasePlanWorkflowSnapshotFields(jobMap, releasePlanWorkflowSnapshotJobRequiredFields...) {
			return false
		}
		if !hasReleasePlanWorkflowSnapshotJobSpecFields(jobMap) {
			return false
		}
	}
	return true
}

func hasReleasePlanWorkflowSnapshotJobSpecFields(job map[string]interface{}) bool {
	if job == nil {
		return false
	}

	jobType, _ := getStringField(job, "type")
	spec, _ := getMapField(job["spec"])
	switch config.JobType(jobType) {
	case config.JobZadigBuild:
		if hasReleasePlanWorkflowSnapshotKey(spec, "service_and_builds") {
			return hasReleasePlanWorkflowSnapshotKey(spec, "default_service_and_builds") &&
				hasReleasePlanWorkflowSnapshotKey(spec, "service_and_builds_options")
		}
	case config.JobZadigScanning:
		if hasReleasePlanWorkflowSnapshotKey(spec, "scannings") {
			return hasReleasePlanWorkflowSnapshotKey(spec, "scanning_options")
		}
	case config.JobZadigDeploy:
		if env, ok := getStringField(spec, "env"); ok && env != "" {
			return hasReleasePlanWorkflowSnapshotKey(spec, "env_options")
		}
		if hasReleasePlanWorkflowSnapshotKey(spec, "services") {
			return true
		}
	case config.JobNacos:
		if hasReleasePlanWorkflowSnapshotKey(spec, "configSource") {
			return hasReleasePlanWorkflowSnapshotKey(spec, "nacos_filtered_data")
		}
	}
	return true
}

func hasReleasePlanWorkflowSnapshotKey(spec map[string]interface{}, key string) bool {
	if spec == nil {
		return false
	}
	value, exists := spec[key]
	if !exists {
		return false
	}
	if value == nil {
		return false
	}
	if items, ok := value.([]interface{}); ok {
		return len(items) > 0
	}
	return true
}

func hasReleasePlanWorkflowSnapshotFields(value map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if _, exists := value[key]; !exists {
			return false
		}
	}
	return true
}

func hasReleasePlanWorkflowSnapshotStringFields(value map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		fieldValue, ok := getStringField(value, key)
		if !ok || fieldValue == "" {
			return false
		}
	}
	return true
}

func applyReleasePlanWorkflowLatestLookupCompat(spec interface{}, workflowSpec *models.WorkflowReleaseJobSpec) {
	if workflowSpec == nil {
		return
	}

	specMap, ok := getMapField(spec)
	if !ok {
		return
	}

	workflowName := firstReleasePlanWorkflowLookupString(specMap, releasePlanWorkflowLegacyNameKeys...)
	projectName := firstReleasePlanWorkflowLookupString(specMap, releasePlanWorkflowLegacyProjectKeys...)
	if workflowSpec.Workflow == nil {
		if workflowName == "" && projectName == "" {
			return
		}
		workflowSpec.Workflow = &models.WorkflowV4{}
	}
	if workflowSpec.Workflow.Name == "" {
		workflowSpec.Workflow.Name = workflowName
	}
	if workflowSpec.Workflow.Project == "" {
		workflowSpec.Workflow.Project = projectName
	}
}

func firstReleasePlanWorkflowLookupString(input map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := getStringField(input, key); ok {
			return value
		}
	}
	return ""
}

func normalizeReleasePlanWorkflowForController(workflow *models.WorkflowV4) (*models.WorkflowV4, error) {
	if workflow == nil {
		return nil, nil
	}

	raw, err := bson.Marshal(workflow)
	if err != nil {
		return nil, err
	}

	generic := bson.M{}
	if err := bson.UnmarshalWithRegistry(releasePlanWorkflowControllerBSONRegistry, raw, &generic); err != nil {
		return nil, err
	}

	resp := new(models.WorkflowV4)
	if err := models.IToi(generic, resp); err != nil {
		return nil, err
	}
	return resp, nil
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
	for _, key := range releasePlanWorkflowSnapshotTopLevelFields {
		if value, exists := workflowMap[key]; exists {
			resp[key] = value
		}
	}
	if params, exists := workflowMap["params"]; exists {
		resp["params"] = filterReleasePlanWorkflowInputValueAtPath("params", params)
	}
	if customField, exists := workflowMap["custom_field"]; exists {
		if filtered := filterReleasePlanWorkflowInputValueAtPath("custom_field", customField); filtered != nil {
			resp["custom_field"] = filtered
		}
	}
	if stages, exists := workflowMap["stages"]; exists {
		resp["stages"] = buildReleasePlanWorkflowStagesInputSnapshot("stages", stages)
	}
	if jobs, exists := workflowMap["jobs"]; exists {
		resp["jobs"] = buildReleasePlanWorkflowJobsInputSnapshot("jobs", jobs)
	}
	return sanitizeReleasePlanGenericValue("", resp), nil
}

func buildReleasePlanWorkflowStagesInputSnapshot(path string, value interface{}) interface{} {
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
		for _, key := range releasePlanWorkflowSnapshotStageDisplayFields {
			if value, exists := stageMap[key]; exists {
				stageResp[key] = filterReleasePlanWorkflowInputValueAtPath(joinReleasePlanWorkflowInputPath(path, key), value)
			}
		}
		if jobs, exists := stageMap["jobs"]; exists {
			stageResp["jobs"] = buildReleasePlanWorkflowJobsInputSnapshot(joinReleasePlanWorkflowInputPath(path, "jobs"), jobs)
		}
		if len(stageResp) > 0 {
			resp = append(resp, stageResp)
		}
	}
	return resp
}

func buildReleasePlanWorkflowJobsInputSnapshot(path string, value interface{}) interface{} {
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
		for _, key := range releasePlanWorkflowSnapshotJobDisplayFields {
			if item, exists := jobMap[key]; exists {
				jobResp[key] = filterReleasePlanWorkflowInputValueAtPath(joinReleasePlanWorkflowInputPath(path, key), item)
			}
		}
		if serviceModules, exists := jobMap["service_modules"]; exists {
			jobResp["service_modules"] = filterReleasePlanWorkflowInputValueAtPath(joinReleasePlanWorkflowInputPath(path, "service_modules"), serviceModules)
		}
		if spec, exists := jobMap["spec"]; exists {
			jobResp["spec"] = filterReleasePlanWorkflowInputValueAtPath(joinReleasePlanWorkflowInputPath(path, "spec"), spec)
		}
		if len(jobResp) > 0 {
			resp = append(resp, jobResp)
		}
	}
	return resp
}

func filterReleasePlanWorkflowInputValue(value interface{}) interface{} {
	return filterReleasePlanWorkflowInputValueAtPath("", value)
}

func filterReleasePlanWorkflowInputValueAtPath(path string, value interface{}) interface{} {
	switch typedValue := value.(type) {
	case map[string]interface{}:
		resp := make(map[string]interface{}, len(typedValue))
		for key, item := range typedValue {
			if key == "plugin" {
				filteredPlugin := filterReleasePlanPluginTemplateInputValueAtPath(joinReleasePlanWorkflowInputPath(path, key), item)
				if filteredPlugin != nil {
					resp[key] = filteredPlugin
				}
				continue
			}
			if shouldDropReleasePlanWorkflowInputField(key) {
				continue
			}
			if key == "variable_yaml" && hasReleasePlanWorkflowStructuredVariables(typedValue) {
				continue
			}
			resp[key] = filterReleasePlanWorkflowInputValueAtPath(joinReleasePlanWorkflowInputPath(path, key), item)
		}
		return resp
	case []interface{}:
		resp := make([]interface{}, 0, len(typedValue))
		for _, item := range typedValue {
			resp = append(resp, filterReleasePlanWorkflowInputValueAtPath(path, item))
		}
		stabilizeReleasePlanWorkflowInputArray(path, resp)
		return resp
	default:
		return value
	}
}

func filterReleasePlanPluginTemplateInputValue(value interface{}) interface{} {
	return filterReleasePlanPluginTemplateInputValueAtPath("plugin", value)
}

func filterReleasePlanPluginTemplateInputValueAtPath(path string, value interface{}) interface{} {
	plugin, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}

	inputs, exists := plugin["inputs"]
	if !exists {
		return nil
	}

	return map[string]interface{}{
		"inputs": filterReleasePlanWorkflowInputValueAtPath(joinReleasePlanWorkflowInputPath(path, "inputs"), inputs),
	}
}

func hasReleasePlanWorkflowStructuredVariables(value map[string]interface{}) bool {
	if value == nil {
		return false
	}
	variableKVs, ok := value["variable_kvs"].([]interface{})
	return ok && len(variableKVs) > 0
}

func stabilizeReleasePlanWorkflowInputArray(path string, items []interface{}) {
	if len(items) < 2 {
		return
	}

	// Only normalize collection-like arrays here. Execution-order arrays such as
	// workflow stages/jobs are intentionally left untouched for display fidelity.
	switch {
	case path == "env_options" || strings.HasSuffix(path, ".env_options"):
		sortReleasePlanWorkflowInputArray(items, releasePlanWorkflowInputArrayKeyByEnv)
	case path == "services" || strings.HasSuffix(path, ".services"):
		sortReleasePlanWorkflowInputArray(items, releasePlanWorkflowInputArrayKeyByService)
	case path == "service_modules" || strings.HasSuffix(path, ".service_modules"):
		sortReleasePlanWorkflowInputArray(items, releasePlanWorkflowInputArrayKeyByServiceModule)
	case path == "modules" || strings.HasSuffix(path, ".modules"):
		sortReleasePlanWorkflowInputArray(items, releasePlanWorkflowInputArrayKeyByModule)
	case path == "variable_kvs" || strings.HasSuffix(path, ".variable_kvs"):
		sortReleasePlanWorkflowInputArray(items, releasePlanWorkflowInputArrayKeyByVariable)
	case path == "target_services" || strings.HasSuffix(path, ".target_services"):
		sortReleasePlanWorkflowInputStringArray(items)
	case path == "service_and_builds" || strings.HasSuffix(path, ".service_and_builds"),
		path == "default_service_and_builds" || strings.HasSuffix(path, ".default_service_and_builds"),
		path == "service_and_builds_options" || strings.HasSuffix(path, ".service_and_builds_options"),
		path == "service_and_images" || strings.HasSuffix(path, ".service_and_images"):
		sortReleasePlanWorkflowInputArray(items, releasePlanWorkflowInputArrayKeyByServiceBuild)
	case path == "service_and_scannings" || strings.HasSuffix(path, ".service_and_scannings"),
		path == "service_scanning_options" || strings.HasSuffix(path, ".service_scanning_options"),
		path == "scannings" || strings.HasSuffix(path, ".scannings"),
		path == "scanning_options" || strings.HasSuffix(path, ".scanning_options"):
		sortReleasePlanWorkflowInputArray(items, releasePlanWorkflowInputArrayKeyByScanning)
	case path == "nacos_filtered_data" || strings.HasSuffix(path, ".nacos_filtered_data"):
		sortReleasePlanWorkflowInputArray(items, releasePlanWorkflowInputArrayKeyByNacosData)
	}
}

func sortReleasePlanWorkflowInputArray(items []interface{}, buildKey func(interface{}) (string, bool)) {
	type sortableItem struct {
		item       interface{}
		primaryKey string
		tieBreak   string
	}

	sortableItems := make([]sortableItem, 0, len(items))
	for _, item := range items {
		primaryKey, ok := buildKey(item)
		if !ok {
			return
		}
		sortableItems = append(sortableItems, sortableItem{item: item, primaryKey: primaryKey})
	}

	sort.SliceStable(sortableItems, func(i, j int) bool {
		return sortableItems[i].primaryKey < sortableItems[j].primaryKey
	})
	for start := 0; start < len(sortableItems); {
		end := start + 1
		for end < len(sortableItems) && sortableItems[end].primaryKey == sortableItems[start].primaryKey {
			end++
		}
		if end-start > 1 {
			for i := start; i < end; i++ {
				hash, err := hashReleasePlanSubtree(sortableItems[i].item)
				if err != nil {
					// Keep the caller's original order intact if we cannot build a stable tie-break key.
					// We return before copying sortableItems back into items, so no partial normalized state leaks out.
					return
				}
				sortableItems[i].tieBreak = hash
			}
			sort.SliceStable(sortableItems[start:end], func(i, j int) bool {
				return sortableItems[start+i].tieBreak < sortableItems[start+j].tieBreak
			})
		}
		start = end
	}
	for i := range sortableItems {
		items[i] = sortableItems[i].item
	}
}

func sortReleasePlanWorkflowInputStringArray(items []interface{}) {
	for _, item := range items {
		if _, ok := item.(string); !ok {
			return
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].(string) < items[j].(string)
	})
}

func releasePlanWorkflowInputArrayKeyByEnv(item interface{}) (string, bool) {
	return releasePlanWorkflowInputArrayKeyByFields(item, "env", "env_name", "env_alias")
}

func releasePlanWorkflowInputArrayKeyByService(item interface{}) (string, bool) {
	return releasePlanWorkflowInputArrayKeyByFields(item, "service_name", "service_module", "image_name")
}

func releasePlanWorkflowInputArrayKeyByServiceModule(item interface{}) (string, bool) {
	return releasePlanWorkflowInputArrayKeyByFields(item, "service_name", "service_module")
}

func releasePlanWorkflowInputArrayKeyByModule(item interface{}) (string, bool) {
	return releasePlanWorkflowInputArrayKeyByFields(item, "service_module", "image_name", "image")
}

func releasePlanWorkflowInputArrayKeyByVariable(item interface{}) (string, bool) {
	return releasePlanWorkflowInputArrayKeyByFields(item, "key")
}

func releasePlanWorkflowInputArrayKeyByServiceBuild(item interface{}) (string, bool) {
	return releasePlanWorkflowInputArrayKeyByFields(item, "service_name", "service_module", "image_name", "build_name", "name")
}

func releasePlanWorkflowInputArrayKeyByScanning(item interface{}) (string, bool) {
	return releasePlanWorkflowInputArrayKeyByFields(item, "service_name", "service_module", "name", "project_name")
}

func releasePlanWorkflowInputArrayKeyByNacosData(item interface{}) (string, bool) {
	return releasePlanWorkflowInputArrayKeyByFields(item, "namespace_id", "group", "data_id")
}

func releasePlanWorkflowInputArrayKeyByFields(item interface{}, keys ...string) (string, bool) {
	value, ok := getMapField(item)
	if !ok {
		return "", false
	}

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		part, exists := getStringField(value, key)
		if exists {
			parts = append(parts, part)
			continue
		}
		if number, exists := getNumberFieldString(value, key); exists {
			parts = append(parts, number)
			continue
		}
		parts = append(parts, "")
	}

	// Keep empty placeholders so keys from heterogeneous-but-compatible items
	// still compare in a consistent field order.
	if strings.TrimSpace(strings.Join(parts, "")) == "" {
		return "", false
	}
	return strings.Join(parts, "|"), true
}

func joinReleasePlanWorkflowInputPath(base, key string) string {
	if key == "" {
		return base
	}
	if base == "" {
		return key
	}
	return base + "." + key
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
