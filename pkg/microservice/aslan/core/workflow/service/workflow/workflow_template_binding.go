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

package workflow

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"go.uber.org/zap"
	jsonpatchdiff "gomodules.xyz/jsonpatch/v2"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"github.com/koderover/zadig/v2/pkg/util"
)

type WorkflowTemplateBindingStatusResponse struct {
	Enabled             bool                               `json:"enabled"`
	TemplateID          string                             `json:"template_id,omitempty"`
	TemplateName        string                             `json:"template_name,omitempty"`
	BaseVersion         int                                `json:"base_version,omitempty"`
	LatestVersion       int                                `json:"latest_version,omitempty"`
	HasDelta            bool                               `json:"has_workflow_delta"`
	HasConflict         bool                               `json:"has_conflict"`
	ConflictCount       int                                `json:"conflict_count"`
	Conflicts           []*WorkflowTemplateBindingConflict `json:"conflicts,omitempty"`
	InvalidPatches      []*commonmodels.InvalidJSONPatch   `json:"invalid_patches,omitempty"`
	RebasedDeltaPatches []*commonmodels.JSONPatchOperation `json:"rebased_delta_patches"`
}

type WorkflowTemplateBindingPreviewResponse struct {
	RenderedWorkflow *commonmodels.WorkflowV4              `json:"rendered_workflow"`
	TemplateBinding  *commonmodels.WorkflowTemplateBinding `json:"template_binding"`
	Conflicts        []*WorkflowTemplateBindingConflict    `json:"conflicts"`
	InvalidPatches   []*commonmodels.InvalidJSONPatch      `json:"invalid_patches"`
}

type WorkflowTemplateReferenceResponse struct {
	TemplateID string                               `json:"template_id"`
	Workflows  []*WorkflowTemplateReferenceWorkflow `json:"workflows"`
}

type WorkflowTemplateReferenceWorkflow struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Project     string `json:"project"`
}

type WorkflowTemplateBindingConflict struct {
	Path string `json:"path"`
}

type ResolveWorkflowTemplateBindingRequest struct {
	TargetVersion int                                `json:"target_version"`
	DeltaPatches  []*commonmodels.JSONPatchOperation `json:"delta_patches"`
}

type UnbindWorkflowTemplateBindingRequest struct {
	SnapshotVersion string `json:"snapshot_version"`
}

func ListWorkflowTemplateVersions(templateID string) ([]*commonmodels.WorkflowV4TemplateVersion, error) {
	return commonrepo.NewWorkflowV4TemplateVersionColl().List(templateID)
}

func DiffWorkflowTemplateVersions(templateID string, fromVersion, toVersion int) ([]*commonmodels.JSONPatchOperation, error) {
	from, err := commonrepo.NewWorkflowV4TemplateVersionColl().Find(&commonrepo.WorkflowTemplateVersionQueryOption{
		TemplateID: templateID,
		Version:    fromVersion,
	})
	if err != nil {
		return nil, err
	}
	to, err := commonrepo.NewWorkflowV4TemplateVersionColl().Find(&commonrepo.WorkflowTemplateVersionQueryOption{
		TemplateID: templateID,
		Version:    toVersion,
	})
	if err != nil {
		return nil, err
	}
	patches, err := createJSONPatchOperations(from.Snapshot, to.Snapshot)
	if err != nil {
		return nil, err
	}
	return filterTemplateBindingPatches(patches), nil
}

func ListWorkflowTemplateReferences(templateID string) (*WorkflowTemplateReferenceResponse, error) {
	workflows, err := commonrepo.NewWorkflowV4Coll().ListTemplateBoundByTemplateID(templateID)
	if err != nil {
		return nil, err
	}
	resp := &WorkflowTemplateReferenceResponse{
		TemplateID: templateID,
		Workflows:  make([]*WorkflowTemplateReferenceWorkflow, 0, len(workflows)),
	}
	for _, workflow := range workflows {
		resp.Workflows = append(resp.Workflows, WorkflowTemplateReferenceFromWorkflow(workflow))
	}
	return resp, nil
}

func WorkflowTemplateReferenceFromWorkflow(workflow *commonmodels.WorkflowV4) *WorkflowTemplateReferenceWorkflow {
	if workflow == nil {
		return nil
	}
	return &WorkflowTemplateReferenceWorkflow{
		Name:        workflow.Name,
		DisplayName: workflow.DisplayName,
		Project:     workflow.Project,
	}
}

func prepareTemplateBoundWorkflowForCreate(user string, workflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	if !isTemplateBindingEnabled(workflow) {
		return nil
	}

	template, version, err := getTemplateAndVersionForBinding(workflow.TemplateBinding.TemplateID, workflow.TemplateBinding.BaseVersion)
	if err != nil {
		return err
	}

	base := workflowFromTemplateSnapshot(version.Snapshot, workflow)
	delta, rendered, err := validateFrontendWorkflowDelta(base, workflow.TemplateBinding.DeltaPatches)
	if err != nil {
		return err
	}

	workflow.TemplateBinding.TemplateID = template.ID.Hex()
	workflow.TemplateBinding.TemplateName = template.TemplateName
	workflow.TemplateBinding.BaseVersion = version.Version
	workflow.TemplateBinding.BaseVersionID = version.ID.Hex()
	workflow.TemplateBinding.DeltaPatches = delta
	workflow.TemplateBinding.ConflictCount = 0
	workflow.TemplateBinding.InvalidPatches = nil

	// the selected snapshot plus the delta is the only source of the inherited fields,
	// so the caller validates and saves exactly what was rendered here.
	applyTemplateBindingInheritedFields(workflow, rendered)
	return nil
}

// With lockTemplateVersion the current base version is kept, which creation uses so the selected version is not upgraded.
func prepareTemplateBoundWorkflowForUpdate(existing, input *commonmodels.WorkflowV4, lockTemplateVersion bool, user string, logger *zap.SugaredLogger) error {
	if !isTemplateBindingEnabled(existing) {
		return nil
	}
	if input.TemplateBinding == nil {
		return fmt.Errorf("template_binding is required for template bound workflow")
	}
	frontendDeltaPatches := input.TemplateBinding.DeltaPatches
	if frontendDeltaPatches == nil {
		return fmt.Errorf("template_binding.delta_patches is required for template bound workflow")
	}
	input.TemplateBinding = &commonmodels.WorkflowTemplateBinding{}
	_ = util.DeepCopy(input.TemplateBinding, existing.TemplateBinding)

	currentVersion, err := getTemplateVersion(existing.TemplateBinding.TemplateID, existing.TemplateBinding.BaseVersion)
	if err != nil {
		return err
	}
	targetVersion, err := getTemplateVersion(existing.TemplateBinding.TemplateID, 0)
	if err != nil {
		return err
	}

	input.TemplateBinding.DeltaPatches = frontendDeltaPatches
	conflicts := calculateBindingConflicts(input, targetVersion.Version)
	if lockTemplateVersion || targetVersion.Version <= currentVersion.Version || len(conflicts) > 0 {
		targetVersion = currentVersion
	}

	base := workflowFromTemplateSnapshot(targetVersion.Snapshot, input)
	delta, _, err := validateFrontendWorkflowDelta(base, frontendDeltaPatches)
	if err != nil {
		return err
	}
	input.TemplateBinding.DeltaPatches = delta
	input.TemplateBinding.BaseVersion = targetVersion.Version
	input.TemplateBinding.BaseVersionID = targetVersion.ID.Hex()
	conflicts, invalidPatches := calculateBindingState(input, 0)
	input.TemplateBinding.ConflictCount = len(conflicts)
	input.TemplateBinding.InvalidPatches = invalidPatches

	renderVersion := 0
	if lockTemplateVersion {
		renderVersion = targetVersion.Version
	}
	rendered, _, err := renderWorkflowWithTemplateBinding(input, renderVersion)
	if err != nil {
		return err
	}
	workflowController := controller.CreateWorkflowController(rendered)
	if err := workflowController.Validate(false); err != nil {
		return err
	}
	return nil
}

// resolveTemplateBindingUpdate selects the base version for a complete delta set and renders the inherited fields from it.
func resolveTemplateBindingUpdate(existing, instance *commonmodels.WorkflowV4, deltaPatches []*commonmodels.JSONPatchOperation) (*commonmodels.WorkflowTemplateBinding, *commonmodels.WorkflowV4, error) {
	current, err := getTemplateVersion(existing.TemplateBinding.TemplateID, existing.TemplateBinding.BaseVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get template version %d: %w", existing.TemplateBinding.BaseVersion, err)
	}
	latest, err := commonrepo.NewWorkflowV4TemplateVersionColl().GetLatest(existing.TemplateBinding.TemplateID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get latest template version: %w", err)
	}
	return selectTemplateBindingVersion(existing.TemplateBinding, current, latest, instance, deltaPatches)
}

// selectTemplateBindingVersion validates the delta on the current base version, then moves to the latest version only when
// the relocated delta applies there without conflict; otherwise it stays on the current base and records the conflicts
// and invalid patches against the latest version.
func selectTemplateBindingVersion(existing *commonmodels.WorkflowTemplateBinding, current, latest *commonmodels.WorkflowV4TemplateVersion, instance *commonmodels.WorkflowV4, deltaPatches []*commonmodels.JSONPatchOperation) (*commonmodels.WorkflowTemplateBinding, *commonmodels.WorkflowV4, error) {
	if deltaPatches == nil {
		deltaPatches = make([]*commonmodels.JSONPatchOperation, 0)
	}
	migrate := latest.Version > current.Version

	// Patches are relocated one by one against the states produced by the previous patches on both versions,
	// so an index still points to the same stage or job after earlier patches have shifted the arrays.
	currentRendered := workflowFromTemplateSnapshot(current.Snapshot, instance)
	var latestBase, latestRendered *commonmodels.WorkflowV4
	if migrate {
		latestBase = workflowFromTemplateSnapshot(latest.Snapshot, instance)
		latestRendered = latestBase
	}
	relocated := make([]*commonmodels.JSONPatchOperation, 0, len(deltaPatches))
	relocatedIdentityPaths := make([]string, 0, len(deltaPatches))
	var invalidPatches []*commonmodels.InvalidJSONPatch
	for _, patch := range deltaPatches {
		if err := validateTemplateBindingPatch(patch); err != nil {
			return nil, nil, fmt.Errorf("invalid workflow template delta: %w", err)
		}
		// once a patch cannot be migrated the later states diverge, so migration stops at the first failure
		if migrate && invalidPatches == nil {
			relocatedPatch, err := relocateTemplateBindingPatch(currentRendered, latestRendered, patch)
			if err == nil {
				var next *commonmodels.WorkflowV4
				identityPath := identityJSONPointer(latestRendered.Stages, relocatedPatch.Path)
				if next, err = applySingleJSONPatch(latestRendered, relocatedPatch); err == nil {
					latestRendered = next
					relocated = append(relocated, relocatedPatch)
					relocatedIdentityPaths = append(relocatedIdentityPaths, identityPath)
				}
			}
			if err != nil {
				invalidPatches = []*commonmodels.InvalidJSONPatch{{Patch: patch, Error: err.Error()}}
			}
		}
		next, err := applySingleJSONPatch(currentRendered, patch)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid workflow template delta: %w", err)
		}
		currentRendered = next
	}

	binding := *existing
	binding.BaseVersion = current.Version
	binding.BaseVersionID = current.ID.Hex()
	binding.DeltaPatches = deltaPatches
	binding.ConflictCount = 0
	binding.InvalidPatches = nil
	if !migrate {
		return &binding, currentRendered, nil
	}
	if len(invalidPatches) > 0 {
		binding.InvalidPatches = invalidPatches
		return &binding, currentRendered, nil
	}

	conflicts, err := templateBindingConflictsByIdentity(current, latest, latestBase, latestRendered, relocated, relocatedIdentityPaths)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to calculate template binding conflicts: %w", err)
	}
	if len(conflicts) > 0 {
		binding.ConflictCount = len(conflicts)
		return &binding, currentRendered, nil
	}

	binding.BaseVersion = latest.Version
	binding.BaseVersionID = latest.ID.Hex()
	binding.DeltaPatches = relocated
	return &binding, latestRendered, nil
}

// relocateTemplateBindingPatch maps the stage and job indexes of a patch from one rendered state to the other by stable IDs.
func relocateTemplateBindingPatch(from, to *commonmodels.WorkflowV4, patch *commonmodels.JSONPatchOperation) (*commonmodels.JSONPatchOperation, error) {
	fromTemplate := &commonmodels.WorkflowV4Template{Stages: from.Stages}
	toTemplate := &commonmodels.WorkflowV4Template{Stages: to.Stages}
	relocated := *patch
	path, err := rebaseTemplateBindingJSONPointer(fromTemplate, toTemplate, patch.Path)
	if err != nil {
		return nil, err
	}
	relocated.Path = path
	if patch.From != "" {
		fromPath, err := rebaseTemplateBindingJSONPointer(fromTemplate, toTemplate, patch.From)
		if err != nil {
			return nil, err
		}
		relocated.From = fromPath
	}
	return &relocated, nil
}

// templateBindingConflictsByIdentity reports the delta patches that change what the template changed from base to target.
// Earlier delta patches may shift the stage and job arrays the later ones address, so stage and job indexes are compared
// by stable IDs: the template changes against the target snapshot, each delta patch against the state it was applied to.
func templateBindingConflictsByIdentity(base, target *commonmodels.WorkflowV4TemplateVersion, targetBase, targetRendered *commonmodels.WorkflowV4, deltaPatches []*commonmodels.JSONPatchOperation, deltaIdentityPaths []string) ([]*WorkflowTemplateBindingConflict, error) {
	if len(deltaPatches) == 0 {
		return nil, nil
	}
	alignedBase := alignTemplateSnapshotForConflict(base.Snapshot, target.Snapshot)
	templatePatches, err := createJSONPatchOperations(alignedBase, target.Snapshot)
	if err != nil {
		return nil, err
	}
	templatePatches = filterTemplateBindingPatches(templatePatches)
	conflicts := make([]*WorkflowTemplateBindingConflict, 0)
	for _, templatePatch := range templatePatches {
		templatePath := identityJSONPointer(target.Snapshot.Stages, templatePatch.Path)
		for i, deltaPath := range deltaIdentityPaths {
			if jsonPatchPathConflict(templatePath, deltaPath) && identityJSONPatchValueConflict(targetBase, targetRendered, moreSpecificJSONPatchPath(templatePath, deltaPath)) {
				conflicts = append(conflicts, &WorkflowTemplateBindingConflict{Path: deltaPatches[i].Path})
				break
			}
		}
	}
	return conflicts, nil
}

// identityJSONPatchValueConflict compares the value at an identity path in the template target and in the rendered result.
func identityJSONPatchValueConflict(base, rendered *commonmodels.WorkflowV4, identityPath string) bool {
	var baseValue, renderedValue interface{}
	baseOK, renderedOK := false, false
	if path, ok := indexJSONPointer(base.Stages, identityPath); ok {
		baseValue, baseOK = workflowJSONPointerValue(base, path)
	}
	if path, ok := indexJSONPointer(rendered.Stages, identityPath); ok {
		renderedValue, renderedOK = workflowJSONPointerValue(rendered, path)
	}
	if baseOK != renderedOK {
		return true
	}
	if !baseOK && !renderedOK {
		return false
	}
	return !reflect.DeepEqual(baseValue, renderedValue)
}

const templateEntityIDPrefix = "@"

// identityJSONPointer replaces the stage and job indexes of a /stages path with the IDs of the entities they address.
// Indexes without an addressable entity, such as "-" or an entity without ID, are kept.
func identityJSONPointer(stages []*commonmodels.WorkflowStage, path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if !strings.HasPrefix(path, "/") || len(parts) < 2 || parts[0] != "stages" {
		return path
	}
	stageIndex, err := strconv.Atoi(parts[1])
	if err != nil || stageIndex < 0 || stageIndex >= len(stages) || stages[stageIndex] == nil || stages[stageIndex].ID == "" {
		return path
	}
	stage := stages[stageIndex]
	parts[1] = templateEntityIDPrefix + stage.ID
	if len(parts) >= 4 && parts[2] == "jobs" {
		jobIndex, err := strconv.Atoi(parts[3])
		if err == nil && jobIndex >= 0 && jobIndex < len(stage.Jobs) && stage.Jobs[jobIndex] != nil && stage.Jobs[jobIndex].ID != "" {
			parts[3] = templateEntityIDPrefix + stage.Jobs[jobIndex].ID
		}
	}
	return "/" + strings.Join(parts, "/")
}

// indexJSONPointer resolves the IDs of an identity path back to indexes in the given stages.
// It returns false when an addressed stage or job does not exist there.
func indexJSONPointer(stages []*commonmodels.WorkflowStage, identityPath string) (string, bool) {
	parts := strings.Split(strings.TrimPrefix(identityPath, "/"), "/")
	if len(parts) < 2 || parts[0] != "stages" || !strings.HasPrefix(parts[1], templateEntityIDPrefix) {
		return identityPath, true
	}
	stageID := strings.TrimPrefix(parts[1], templateEntityIDPrefix)
	stageIndex := -1
	for i, stage := range stages {
		if stage != nil && stage.ID == stageID {
			stageIndex = i
			break
		}
	}
	if stageIndex < 0 {
		return "", false
	}
	parts[1] = strconv.Itoa(stageIndex)
	if len(parts) >= 4 && parts[2] == "jobs" && strings.HasPrefix(parts[3], templateEntityIDPrefix) {
		jobID := strings.TrimPrefix(parts[3], templateEntityIDPrefix)
		jobIndex := -1
		for i, job := range stages[stageIndex].Jobs {
			if job != nil && job.ID == jobID {
				jobIndex = i
				break
			}
		}
		if jobIndex < 0 {
			return "", false
		}
		parts[3] = strconv.Itoa(jobIndex)
	}
	return "/" + strings.Join(parts, "/"), true
}

// applyTemplateBindingInheritedFields copies the fields inherited from the template from the rendered workflow.
func applyTemplateBindingInheritedFields(workflow, rendered *commonmodels.WorkflowV4) {
	workflow.TemplateName = rendered.TemplateName
	workflow.Category = rendered.Category
	workflow.Params = rendered.Params
	workflow.Stages = rendered.Stages
	workflow.ShareStorages = rendered.ShareStorages
	workflow.ConcurrencyLimit = rendered.ConcurrencyLimit
}

// renderTemplateBoundWorkflowOnBase renders a template bound workflow on its base version and fails on any invalid delta.
func renderTemplateBoundWorkflowOnBase(workflow *commonmodels.WorkflowV4) (*commonmodels.WorkflowV4, error) {
	version, err := getTemplateVersion(workflow.TemplateBinding.TemplateID, workflow.TemplateBinding.BaseVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get template version %d: %w", workflow.TemplateBinding.BaseVersion, err)
	}
	_, rendered, err := validateFrontendWorkflowDelta(workflowFromTemplateSnapshot(version.Snapshot, workflow), workflow.TemplateBinding.DeltaPatches)
	if err != nil {
		return nil, fmt.Errorf("failed to render workflow on template version %d: %w", version.Version, err)
	}
	return rendered, nil
}

func RenderWorkflowV4WithTemplateBinding(workflow *commonmodels.WorkflowV4) (*commonmodels.WorkflowV4, *WorkflowTemplateBindingPreviewResponse, error) {
	return renderWorkflowWithTemplateBinding(workflow, workflow.TemplateBinding.BaseVersion)
}

func GetWorkflowTemplateBindingStatus(workflowName string) (*WorkflowTemplateBindingStatusResponse, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		return nil, err
	}
	if !isTemplateBindingEnabled(workflow) {
		return &WorkflowTemplateBindingStatusResponse{Enabled: false}, nil
	}

	latestVersion := workflow.TemplateBinding.BaseVersion
	rebasedPatches := workflow.TemplateBinding.DeltaPatches
	conflicts, invalidPatches := calculateBindingState(workflow, 0)
	if latest, err := commonrepo.NewWorkflowV4TemplateVersionColl().GetLatest(workflow.TemplateBinding.TemplateID); err == nil {
		latestVersion = latest.Version
		base, err := getTemplateVersion(workflow.TemplateBinding.TemplateID, workflow.TemplateBinding.BaseVersion)
		if err != nil {
			return nil, err
		}
		relocatedPatches, rebaseInvalid := rebaseTemplateBindingPatches(base.Snapshot, latest.Snapshot, workflow.TemplateBinding.DeltaPatches)
		targetBase := workflowFromTemplateSnapshot(latest.Snapshot, workflow)
		var applyInvalid []*commonmodels.InvalidJSONPatch
		rebasedPatches, applyInvalid = filterApplicableWorkflowDeltaPatches(targetBase, relocatedPatches)

		rebasedWorkflow := new(commonmodels.WorkflowV4)
		if err := util.DeepCopy(rebasedWorkflow, workflow); err != nil {
			return nil, err
		}
		rebasedWorkflow.TemplateBinding.DeltaPatches = rebasedPatches
		conflicts = calculateBindingConflicts(rebasedWorkflow, latest.Version)
		invalidPatches = append(rebaseInvalid, applyInvalid...)
	}
	return &WorkflowTemplateBindingStatusResponse{
		Enabled:             true,
		TemplateID:          workflow.TemplateBinding.TemplateID,
		TemplateName:        workflow.TemplateBinding.TemplateName,
		BaseVersion:         workflow.TemplateBinding.BaseVersion,
		LatestVersion:       latestVersion,
		HasDelta:            len(workflow.TemplateBinding.DeltaPatches) > 0,
		HasConflict:         len(conflicts) > 0,
		ConflictCount:       len(conflicts),
		Conflicts:           conflicts,
		InvalidPatches:      invalidPatches,
		RebasedDeltaPatches: rebasedPatches,
	}, nil
}

func ResolveWorkflowTemplateBinding(workflowName, user string, req *ResolveWorkflowTemplateBindingRequest, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		return err
	}
	if !isTemplateBindingEnabled(workflow) {
		return fmt.Errorf("workflow %s is not template bound", workflowName)
	}
	if req == nil {
		req = &ResolveWorkflowTemplateBindingRequest{}
	}
	if req.DeltaPatches == nil {
		return fmt.Errorf("delta_patches is required to resolve workflow template binding")
	}
	targetVersion := req.TargetVersion
	if targetVersion == 0 {
		latest, err := commonrepo.NewWorkflowV4TemplateVersionColl().GetLatest(workflow.TemplateBinding.TemplateID)
		if err != nil {
			return err
		}
		targetVersion = latest.Version
	}
	target, err := getTemplateVersion(workflow.TemplateBinding.TemplateID, targetVersion)
	if err != nil {
		return err
	}
	base := workflowFromTemplateSnapshot(target.Snapshot, workflow)
	delta, _, err := validateFrontendWorkflowDelta(base, req.DeltaPatches)
	if err != nil {
		return err
	}
	workflow.TemplateBinding.DeltaPatches = delta

	workflow.TemplateBinding.BaseVersion = target.Version
	workflow.TemplateBinding.BaseVersionID = target.ID.Hex()
	workflow.TemplateBinding.ConflictCount = 0
	workflow.TemplateBinding.InvalidPatches = nil
	workflow.UpdatedBy = user
	workflow.UpdateTime = time.Now().Unix()
	return commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow)
}

func UnbindWorkflowTemplateBinding(workflowName, user string, req *UnbindWorkflowTemplateBindingRequest, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		return err
	}
	if !isTemplateBindingEnabled(workflow) {
		return nil
	}
	rendered, _, err := renderWorkflowWithTemplateBinding(workflow, 0)
	if err != nil {
		return err
	}
	rendered.TemplateBinding = &commonmodels.WorkflowTemplateBinding{Enabled: false}
	clearWorkflowStageAndJobIDs(rendered.Stages)
	rendered.ID = workflow.ID
	rendered.CustomField = workflow.CustomField
	rendered.UpdatedBy = user
	rendered.UpdateTime = time.Now().Unix()
	return commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), rendered)
}

func renderWorkflowWithTemplateBinding(workflow *commonmodels.WorkflowV4, targetVersion int) (*commonmodels.WorkflowV4, *WorkflowTemplateBindingPreviewResponse, error) {
	if !isTemplateBindingEnabled(workflow) {
		return workflow, &WorkflowTemplateBindingPreviewResponse{RenderedWorkflow: workflow}, nil
	}

	version, err := getTemplateVersion(workflow.TemplateBinding.TemplateID, targetVersion)
	if err != nil {
		return nil, nil, err
	}
	rendered := workflowFromTemplateSnapshot(version.Snapshot, workflow)
	invalidPatches := make([]*commonmodels.InvalidJSONPatch, 0)
	for _, patch := range workflow.TemplateBinding.DeltaPatches {
		next, err := applySingleJSONPatch(rendered, patch)
		if err != nil {
			invalidPatches = append(invalidPatches, &commonmodels.InvalidJSONPatch{Patch: patch, Error: err.Error()})
			continue
		}
		rendered = next
	}
	if rendered.TemplateBinding == nil {
		rendered.TemplateBinding = workflow.TemplateBinding
	}
	conflicts, calculatedInvalid := calculateBindingState(workflow, targetVersion)
	invalidPatches = append(invalidPatches, calculatedInvalid...)
	rendered.TemplateBinding.ConflictCount = len(conflicts)
	rendered.TemplateBinding.InvalidPatches = invalidPatches
	return rendered, &WorkflowTemplateBindingPreviewResponse{
		RenderedWorkflow: rendered,
		TemplateBinding:  rendered.TemplateBinding,
		Conflicts:        conflicts,
		InvalidPatches:   invalidPatches,
	}, nil
}

func calculateBindingState(workflow *commonmodels.WorkflowV4, targetVersion int) ([]*WorkflowTemplateBindingConflict, []*commonmodels.InvalidJSONPatch) {
	conflicts := calculateBindingConflicts(workflow, targetVersion)
	invalidPatches := validateWorkflowDeltaPatches(workflow, targetVersion)
	return conflicts, invalidPatches
}

func calculateBindingConflicts(workflow *commonmodels.WorkflowV4, targetVersion int) []*WorkflowTemplateBindingConflict {
	if !isTemplateBindingEnabled(workflow) || len(workflow.TemplateBinding.DeltaPatches) == 0 {
		return nil
	}
	base, err := getTemplateVersion(workflow.TemplateBinding.TemplateID, workflow.TemplateBinding.BaseVersion)
	if err != nil {
		return nil
	}
	target, err := getTemplateVersion(workflow.TemplateBinding.TemplateID, targetVersion)
	if err != nil {
		return nil
	}
	if target.Version <= base.Version {
		return nil
	}
	alignedBase := alignTemplateSnapshotForConflict(base.Snapshot, target.Snapshot)
	templatePatches, err := createJSONPatchOperations(alignedBase, target.Snapshot)
	if err != nil {
		return nil
	}
	templatePatches = filterTemplateBindingPatches(templatePatches)
	targetBase := workflowFromTemplateSnapshot(target.Snapshot, workflow)
	targetRendered := targetBase
	for _, deltaPatch := range workflow.TemplateBinding.DeltaPatches {
		next, err := applySingleJSONPatch(targetRendered, deltaPatch)
		if err != nil {
			continue
		}
		targetRendered = next
	}
	conflicts := make([]*WorkflowTemplateBindingConflict, 0)
	for _, templatePatch := range templatePatches {
		for _, deltaPatch := range workflow.TemplateBinding.DeltaPatches {
			if jsonPatchPathConflict(templatePatch.Path, deltaPatch.Path) && jsonPatchValueConflict(targetBase, targetRendered, templatePatch.Path, deltaPatch.Path) {
				conflicts = append(conflicts, &WorkflowTemplateBindingConflict{Path: deltaPatch.Path})
				break
			}
		}
	}
	return conflicts
}

func alignTemplateSnapshotForConflict(base, target *commonmodels.WorkflowV4Template) *commonmodels.WorkflowV4Template {
	if base == nil || target == nil {
		return base
	}

	aligned := new(commonmodels.WorkflowV4Template)
	if err := util.DeepCopy(aligned, base); err != nil {
		return base
	}
	aligned.Stages = make([]*commonmodels.WorkflowStage, 0, len(target.Stages))
	for _, targetStage := range target.Stages {
		if targetStage == nil {
			aligned.Stages = append(aligned.Stages, nil)
			continue
		}

		baseStage := findMatchingTemplateStage(base.Stages, targetStage)
		if baseStage == nil {
			aligned.Stages = append(aligned.Stages, targetStage)
			continue
		}

		alignedStage := new(commonmodels.WorkflowStage)
		if err := util.DeepCopy(alignedStage, baseStage); err != nil {
			aligned.Stages = append(aligned.Stages, targetStage)
			continue
		}
		if targetStage.Jobs == nil {
			alignedStage.Jobs = nil
			aligned.Stages = append(aligned.Stages, alignedStage)
			continue
		}
		alignedStage.Jobs = make([]*commonmodels.Job, 0, len(targetStage.Jobs))
		for _, targetJob := range targetStage.Jobs {
			if targetJob == nil {
				alignedStage.Jobs = append(alignedStage.Jobs, nil)
				continue
			}
			if baseJob := findMatchingTemplateJob(base.Stages, targetJob); baseJob != nil {
				alignedStage.Jobs = append(alignedStage.Jobs, baseJob)
			} else {
				alignedStage.Jobs = append(alignedStage.Jobs, targetJob)
			}
		}
		aligned.Stages = append(aligned.Stages, alignedStage)
	}
	return aligned
}

func findMatchingTemplateStage(stages []*commonmodels.WorkflowStage, target *commonmodels.WorkflowStage) *commonmodels.WorkflowStage {
	for _, stage := range stages {
		if stage != nil && sameTemplateEntity(stage.ID, target.ID) {
			return stage
		}
	}
	return nil
}

func findMatchingTemplateJob(stages []*commonmodels.WorkflowStage, target *commonmodels.Job) *commonmodels.Job {
	for _, stage := range stages {
		if stage == nil {
			continue
		}
		for _, job := range stage.Jobs {
			if job != nil && sameTemplateEntity(job.ID, target.ID) {
				return job
			}
		}
	}
	return nil
}

func sameTemplateEntity(aID, bID string) bool {
	return aID != "" && bID != "" && aID == bID
}

func validateWorkflowDeltaPatches(workflow *commonmodels.WorkflowV4, targetVersion int) []*commonmodels.InvalidJSONPatch {
	if !isTemplateBindingEnabled(workflow) {
		return nil
	}
	version, err := getTemplateVersion(workflow.TemplateBinding.TemplateID, targetVersion)
	if err != nil {
		return nil
	}
	rendered := workflowFromTemplateSnapshot(version.Snapshot, workflow)
	invalidPatches := make([]*commonmodels.InvalidJSONPatch, 0)
	for _, patch := range workflow.TemplateBinding.DeltaPatches {
		next, err := applySingleJSONPatch(rendered, patch)
		if err != nil {
			invalidPatches = append(invalidPatches, &commonmodels.InvalidJSONPatch{Patch: patch, Error: err.Error()})
			continue
		}
		rendered = next
	}
	return invalidPatches
}

func getTemplateAndVersionForBinding(templateID string, version int) (*commonmodels.WorkflowV4Template, *commonmodels.WorkflowV4TemplateVersion, error) {
	template, err := commonrepo.NewWorkflowV4TemplateColl().Find(&commonrepo.WorkflowTemplateQueryOption{ID: templateID})
	if err != nil {
		return nil, nil, err
	}
	versionModel, err := getTemplateVersion(template.ID.Hex(), version)
	if err != nil {
		return nil, nil, err
	}
	return template, versionModel, nil
}

func getTemplateVersion(templateID string, version int) (*commonmodels.WorkflowV4TemplateVersion, error) {
	if version > 0 {
		return commonrepo.NewWorkflowV4TemplateVersionColl().Find(&commonrepo.WorkflowTemplateVersionQueryOption{
			TemplateID: templateID,
			Version:    version,
		})
	}
	latest, err := commonrepo.NewWorkflowV4TemplateVersionColl().GetLatest(templateID)
	return latest, err
}

func workflowFromTemplateSnapshot(template *commonmodels.WorkflowV4Template, instance *commonmodels.WorkflowV4) *commonmodels.WorkflowV4 {
	rendered := new(commonmodels.WorkflowV4)
	_ = util.DeepCopy(rendered, instance)
	rendered.TemplateName = template.TemplateName
	rendered.Category = template.Category
	rendered.Params = template.Params
	rendered.Stages = template.Stages
	rendered.Description = instance.Description
	rendered.ShareStorages = template.ShareStorages
	rendered.ConcurrencyLimit = template.ConcurrencyLimit
	return rendered
}

func isTemplateBindingEnabled(workflow *commonmodels.WorkflowV4) bool {
	return workflow != nil && workflow.TemplateBinding != nil && workflow.TemplateBinding.Enabled
}

func clearWorkflowStageAndJobIDs(stages []*commonmodels.WorkflowStage) {
	for _, stage := range stages {
		if stage == nil {
			continue
		}
		stage.ID = ""
		for _, job := range stage.Jobs {
			if job != nil {
				job.ID = ""
			}
		}
	}
}

func validateFrontendWorkflowDelta(base *commonmodels.WorkflowV4, patches []*commonmodels.JSONPatchOperation) ([]*commonmodels.JSONPatchOperation, *commonmodels.WorkflowV4, error) {
	if patches == nil {
		patches = make([]*commonmodels.JSONPatchOperation, 0)
	}
	rendered := base
	for _, patch := range patches {
		if err := validateTemplateBindingPatch(patch); err != nil {
			return nil, nil, err
		}
		next, err := applySingleJSONPatch(rendered, patch)
		if err != nil {
			return nil, nil, err
		}
		rendered = next
	}
	return patches, rendered, nil
}

func validateTemplateBindingPatch(patch *commonmodels.JSONPatchOperation) error {
	if patch == nil {
		return fmt.Errorf("nil json patch operation")
	}
	return nil
}

func createJSONPatchOperations(from, to interface{}) ([]*commonmodels.JSONPatchOperation, error) {
	fromBytes, err := json.Marshal(from)
	if err != nil {
		return nil, err
	}
	toBytes, err := json.Marshal(to)
	if err != nil {
		return nil, err
	}
	operations, err := jsonpatchdiff.CreatePatch(fromBytes, toBytes)
	if err != nil {
		return nil, err
	}
	resp := make([]*commonmodels.JSONPatchOperation, 0, len(operations))
	for _, op := range operations {
		resp = append(resp, &commonmodels.JSONPatchOperation{
			Operation: op.Operation,
			Path:      op.Path,
			Value:     op.Value,
		})
	}
	return resp, nil
}

func applySingleJSONPatch(workflow *commonmodels.WorkflowV4, patch *commonmodels.JSONPatchOperation) (*commonmodels.WorkflowV4, error) {
	workflowBytes, err := json.Marshal(workflow)
	if err != nil {
		return nil, err
	}
	patchBytes, err := json.Marshal([]*commonmodels.JSONPatchOperation{patch})
	if err != nil {
		return nil, err
	}
	decodedPatch, err := jsonpatch.DecodePatch(patchBytes)
	if err != nil {
		return nil, err
	}
	options := jsonpatch.NewApplyOptions()
	options.AllowMissingPathOnRemove = true
	options.EnsurePathExistsOnAdd = false
	renderedBytes, err := decodedPatch.ApplyWithOptions(workflowBytes, options)
	if err != nil {
		return nil, err
	}
	rendered := new(commonmodels.WorkflowV4)
	if err := json.Unmarshal(renderedBytes, rendered); err != nil {
		return nil, err
	}
	return rendered, nil
}

func rebaseTemplateBindingPatches(from, to *commonmodels.WorkflowV4Template, patches []*commonmodels.JSONPatchOperation) ([]*commonmodels.JSONPatchOperation, []*commonmodels.InvalidJSONPatch) {
	rebased := make([]*commonmodels.JSONPatchOperation, 0, len(patches))
	invalid := make([]*commonmodels.InvalidJSONPatch, 0)
	for _, patch := range patches {
		if patch == nil {
			invalid = append(invalid, &commonmodels.InvalidJSONPatch{Patch: patch, Error: "nil json patch operation"})
			continue
		}

		rebasedPatch := *patch
		path, err := rebaseTemplateBindingJSONPointer(from, to, patch.Path)
		if err != nil {
			invalid = append(invalid, &commonmodels.InvalidJSONPatch{Patch: patch, Error: err.Error()})
			continue
		}
		rebasedPatch.Path = path
		if patch.From != "" {
			fromPath, err := rebaseTemplateBindingJSONPointer(from, to, patch.From)
			if err != nil {
				invalid = append(invalid, &commonmodels.InvalidJSONPatch{Patch: patch, Error: err.Error()})
				continue
			}
			rebasedPatch.From = fromPath
		}
		rebased = append(rebased, &rebasedPatch)
	}
	return rebased, invalid
}

func filterApplicableWorkflowDeltaPatches(base *commonmodels.WorkflowV4, patches []*commonmodels.JSONPatchOperation) ([]*commonmodels.JSONPatchOperation, []*commonmodels.InvalidJSONPatch) {
	applicable := make([]*commonmodels.JSONPatchOperation, 0, len(patches))
	invalid := make([]*commonmodels.InvalidJSONPatch, 0)
	rendered := base
	for _, patch := range patches {
		next, err := applySingleJSONPatch(rendered, patch)
		if err != nil {
			invalid = append(invalid, &commonmodels.InvalidJSONPatch{Patch: patch, Error: err.Error()})
			continue
		}
		applicable = append(applicable, patch)
		rendered = next
	}
	return applicable, invalid
}

func rebaseTemplateBindingJSONPointer(from, to *commonmodels.WorkflowV4Template, path string) (string, error) {
	if from == nil || to == nil || path == "" {
		return path, nil
	}

	rawParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if !strings.HasPrefix(path, "/") || len(rawParts) < 2 || rawParts[0] != "stages" {
		return path, nil
	}
	stageIndex, err := strconv.Atoi(rawParts[1])
	if err != nil {
		// Paths such as /stages/- add a new entity and do not refer to an
		// existing template stage, so there is nothing to relocate.
		return path, nil
	}
	if stageIndex < 0 || stageIndex >= len(from.Stages) || from.Stages[stageIndex] == nil {
		return "", fmt.Errorf("stage at index %d does not exist in base template", stageIndex)
	}

	sourceStage := from.Stages[stageIndex]
	if sourceStage.ID == "" {
		return "", fmt.Errorf("stage %q has no id in base template", sourceStage.Name)
	}
	targetStageIndex := findTemplateStageIndex(to.Stages, sourceStage)
	if targetStageIndex < 0 {
		return "", fmt.Errorf("stage %q no longer exists in target template", templateEntityIdentity(sourceStage.ID, sourceStage.Name))
	}
	rawParts[1] = strconv.Itoa(targetStageIndex)

	if len(rawParts) < 4 || rawParts[2] != "jobs" {
		return "/" + strings.Join(rawParts, "/"), nil
	}
	jobIndex, err := strconv.Atoi(rawParts[3])
	if err != nil {
		return "/" + strings.Join(rawParts, "/"), nil
	}
	if jobIndex < 0 || jobIndex >= len(sourceStage.Jobs) || sourceStage.Jobs[jobIndex] == nil {
		return "", fmt.Errorf("job at index %d does not exist in base template stage %q", jobIndex, templateEntityIdentity(sourceStage.ID, sourceStage.Name))
	}

	sourceJob := sourceStage.Jobs[jobIndex]
	if sourceJob.ID == "" {
		return "", fmt.Errorf("job %q has no id in base template", sourceJob.Name)
	}
	targetStageIndex, targetJobIndex := findTemplateJobIndex(to.Stages, sourceJob)
	if targetJobIndex < 0 {
		return "", fmt.Errorf("job %q no longer exists in target template", templateEntityIdentity(sourceJob.ID, sourceJob.Name))
	}
	rawParts[1] = strconv.Itoa(targetStageIndex)
	rawParts[3] = strconv.Itoa(targetJobIndex)
	return "/" + strings.Join(rawParts, "/"), nil
}

func findTemplateStageIndex(stages []*commonmodels.WorkflowStage, source *commonmodels.WorkflowStage) int {
	for index, stage := range stages {
		if stage == nil {
			continue
		}
		if stage.ID == source.ID {
			return index
		}
	}
	return -1
}

func findTemplateJobIndex(stages []*commonmodels.WorkflowStage, source *commonmodels.Job) (int, int) {
	for stageIndex, stage := range stages {
		if stage == nil {
			continue
		}
		for jobIndex, job := range stage.Jobs {
			if job != nil && job.ID == source.ID {
				return stageIndex, jobIndex
			}
		}
	}
	return -1, -1
}

func templateEntityIdentity(id, name string) string {
	if id != "" {
		return id
	}
	return name
}

func jsonPatchPathConflict(a, b string) bool {
	if a == b {
		return true
	}
	a = strings.TrimRight(a, "/")
	b = strings.TrimRight(b, "/")
	return strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

func jsonPatchValueConflict(base, rendered *commonmodels.WorkflowV4, templatePath, deltaPath string) bool {
	comparePath := moreSpecificJSONPatchPath(templatePath, deltaPath)
	baseValue, baseOK := workflowJSONPointerValue(base, comparePath)
	renderedValue, renderedOK := workflowJSONPointerValue(rendered, comparePath)
	if baseOK != renderedOK {
		return true
	}
	if !baseOK && !renderedOK {
		return false
	}
	return !reflect.DeepEqual(baseValue, renderedValue)
}

func moreSpecificJSONPatchPath(a, b string) string {
	a = strings.TrimRight(a, "/")
	b = strings.TrimRight(b, "/")
	if strings.HasPrefix(a, b+"/") {
		return a
	}
	return b
}

func workflowJSONPointerValue(workflow *commonmodels.WorkflowV4, path string) (interface{}, bool) {
	workflowBytes, err := json.Marshal(workflow)
	if err != nil {
		return nil, false
	}
	var data interface{}
	if err := json.Unmarshal(workflowBytes, &data); err != nil {
		return nil, false
	}
	return jsonPointerValue(data, path)
}

func jsonPointerValue(data interface{}, path string) (interface{}, bool) {
	if path == "" {
		return data, true
	}
	if !strings.HasPrefix(path, "/") {
		return nil, false
	}
	current := data
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch typed := current.(type) {
		case map[string]interface{}:
			next, ok := typed[part]
			if !ok {
				return nil, false
			}
			current = next
		case []interface{}:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func filterTemplateBindingPatches(patches []*commonmodels.JSONPatchOperation) []*commonmodels.JSONPatchOperation {
	resp := make([]*commonmodels.JSONPatchOperation, 0, len(patches))
	for _, patch := range patches {
		if patch == nil {
			continue
		}
		if isTemplateBindingInheritedPath(patch.Path) {
			resp = append(resp, patch)
		}
	}
	return resp
}

func isTemplateBindingInheritedPath(path string) bool {
	return path == "/params" || strings.HasPrefix(path, "/params/") ||
		path == "/stages" || strings.HasPrefix(path, "/stages/") ||
		path == "/share_storages" || strings.HasPrefix(path, "/share_storages/") ||
		path == "/concurrency_limit"
}
