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
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	jsonpatchdiff "gomodules.xyz/jsonpatch/v2"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"github.com/koderover/zadig/v2/pkg/util"
)

const (
	WorkflowTemplateBindingStatusSynced          = "synced"
	WorkflowTemplateBindingStatusTemplateUpdated = "template_updated"
	WorkflowTemplateBindingStatusConflict        = "conflict"
	WorkflowTemplateBindingStatusInvalidDelta    = "invalid_delta"
)

type WorkflowTemplateBindingStatusResponse struct {
	Enabled        bool                               `json:"enabled"`
	TemplateID     string                             `json:"template_id,omitempty"`
	TemplateName   string                             `json:"template_name,omitempty"`
	BaseVersion    int                                `json:"base_version,omitempty"`
	LatestVersion  int                                `json:"latest_version,omitempty"`
	Status         string                             `json:"status,omitempty"`
	HasDelta       bool                               `json:"has_workflow_delta"`
	HasConflict    bool                               `json:"has_conflict"`
	ConflictCount  int                                `json:"conflict_count"`
	Conflicts      []*WorkflowTemplateBindingConflict `json:"conflicts,omitempty"`
	InvalidPatches []*commonmodels.InvalidJSONPatch   `json:"invalid_patches,omitempty"`
}

type WorkflowTemplateBindingPreviewResponse struct {
	RenderedWorkflow *commonmodels.WorkflowV4              `json:"rendered_workflow"`
	TemplateBinding  *commonmodels.WorkflowTemplateBinding `json:"template_binding"`
	Conflicts        []*WorkflowTemplateBindingConflict    `json:"conflicts"`
	InvalidPatches   []*commonmodels.InvalidJSONPatch      `json:"invalid_patches"`
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

func EnsureWorkflowTemplateVersion(templateID, user string) (*commonmodels.WorkflowV4TemplateVersion, error) {
	template, err := commonrepo.NewWorkflowV4TemplateColl().Find(&commonrepo.WorkflowTemplateQueryOption{ID: templateID})
	if err != nil {
		return nil, err
	}
	if template.LatestVersion > 0 && template.LatestVersionID != "" {
		version, err := commonrepo.NewWorkflowV4TemplateVersionColl().Find(&commonrepo.WorkflowTemplateVersionQueryOption{
			VersionID: template.LatestVersionID,
		})
		if err == nil {
			return version, nil
		}
		if err != mongo.ErrNoDocuments {
			return nil, err
		}
	}
	version, err := commonrepo.NewWorkflowV4TemplateVersionColl().CreateNext(template, user)
	if err != nil {
		return nil, err
	}
	template.LatestVersion = version.Version
	template.LatestVersionID = version.ID.Hex()
	if err := commonrepo.NewWorkflowV4TemplateColl().UpdateVersionInfo(template.ID, version.Version, version.ID.Hex()); err != nil {
		return nil, err
	}
	return version, nil
}

func ListWorkflowTemplateVersions(templateID string) ([]*commonmodels.WorkflowV4TemplateVersion, error) {
	if _, err := EnsureWorkflowTemplateVersion(templateID, ""); err != nil {
		return nil, err
	}
	return commonrepo.NewWorkflowV4TemplateVersionColl().List(templateID)
}

func DiffWorkflowTemplateVersions(templateID string, fromVersion, toVersion int) ([]*commonmodels.JSONPatchOperation, error) {
	if _, err := EnsureWorkflowTemplateVersion(templateID, ""); err != nil {
		return nil, err
	}
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
	return createJSONPatchOperations(from.Snapshot, to.Snapshot)
}

func prepareTemplateBoundWorkflowForCreate(user string, workflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	if !isTemplateBindingEnabled(workflow) {
		return nil
	}

	template, version, err := getTemplateAndVersionForBinding(workflow.TemplateBinding.TemplateID, workflow.TemplateBinding.BaseVersion, user)
	if err != nil {
		return err
	}

	base := workflowFromTemplateSnapshot(version.Snapshot, workflow)
	delta, _, err := validateFrontendWorkflowDelta(base, workflow.TemplateBinding.DeltaPatches)
	if err != nil {
		return err
	}

	workflow.TemplateBinding.TemplateID = template.ID.Hex()
	workflow.TemplateBinding.TemplateName = template.TemplateName
	workflow.TemplateBinding.BaseVersion = version.Version
	workflow.TemplateBinding.BaseVersionID = version.ID.Hex()
	workflow.TemplateBinding.LatestResolvedVersion = version.Version
	workflow.TemplateBinding.DeltaPatches = delta
	workflow.TemplateBinding.Status = WorkflowTemplateBindingStatusSynced
	workflow.TemplateBinding.ConflictCount = 0
	workflow.TemplateBinding.InvalidPatches = nil

	rendered, preview, err := renderWorkflowWithTemplateBinding(workflow, 0)
	if err != nil {
		return err
	}
	if len(preview.InvalidPatches) > 0 {
		return fmt.Errorf("invalid workflow template delta")
	}
	workflowController := controller.CreateWorkflowController(rendered)
	if err := workflowController.Validate(false); err != nil {
		return err
	}
	return nil
}

func prepareTemplateBoundWorkflowForUpdate(existing, input *commonmodels.WorkflowV4, user string, logger *zap.SugaredLogger) error {
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
	if targetVersion.Version <= currentVersion.Version || len(conflicts) > 0 {
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
	if targetVersion.Version > existing.TemplateBinding.BaseVersion {
		input.TemplateBinding.LatestResolvedVersion = targetVersion.Version
	} else {
		input.TemplateBinding.LatestResolvedVersion = existing.TemplateBinding.LatestResolvedVersion
	}
	status, conflicts, invalidPatches := calculateBindingStatus(input, 0)
	input.TemplateBinding.Status = status
	input.TemplateBinding.ConflictCount = len(conflicts)
	input.TemplateBinding.InvalidPatches = invalidPatches

	rendered, _, err := renderWorkflowWithTemplateBinding(input, 0)
	if err != nil {
		return err
	}
	workflowController := controller.CreateWorkflowController(rendered)
	if err := workflowController.Validate(false); err != nil {
		return err
	}
	return nil
}

func RenderWorkflowV4WithTemplateBinding(workflow *commonmodels.WorkflowV4) (*commonmodels.WorkflowV4, *WorkflowTemplateBindingPreviewResponse, error) {
	return renderWorkflowWithTemplateBinding(workflow, 0)
}

func GetWorkflowTemplateBindingStatus(workflowName string) (*WorkflowTemplateBindingStatusResponse, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		return nil, err
	}
	if !isTemplateBindingEnabled(workflow) {
		return &WorkflowTemplateBindingStatusResponse{Enabled: false}, nil
	}

	status, conflicts, invalidPatches := calculateBindingStatus(workflow, 0)
	latestVersion := workflow.TemplateBinding.BaseVersion
	if latest, err := commonrepo.NewWorkflowV4TemplateVersionColl().GetLatest(workflow.TemplateBinding.TemplateID); err == nil {
		latestVersion = latest.Version
	}
	return &WorkflowTemplateBindingStatusResponse{
		Enabled:        true,
		TemplateID:     workflow.TemplateBinding.TemplateID,
		TemplateName:   workflow.TemplateBinding.TemplateName,
		BaseVersion:    workflow.TemplateBinding.BaseVersion,
		LatestVersion:  latestVersion,
		Status:         status,
		HasDelta:       len(workflow.TemplateBinding.DeltaPatches) > 0,
		HasConflict:    len(conflicts) > 0,
		ConflictCount:  len(conflicts),
		Conflicts:      conflicts,
		InvalidPatches: invalidPatches,
	}, nil
}

func PreviewWorkflowTemplateBinding(workflowName string, targetVersion int) (*WorkflowTemplateBindingPreviewResponse, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		return nil, err
	}
	if !isTemplateBindingEnabled(workflow) {
		return &WorkflowTemplateBindingPreviewResponse{RenderedWorkflow: workflow}, nil
	}
	_, preview, err := renderWorkflowWithTemplateBinding(workflow, targetVersion)
	return preview, err
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
	workflow.TemplateBinding.LatestResolvedVersion = target.Version
	workflow.TemplateBinding.Status = WorkflowTemplateBindingStatusSynced
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
	status, conflicts, calculatedInvalid := calculateBindingStatus(workflow, targetVersion)
	invalidPatches = append(invalidPatches, calculatedInvalid...)
	rendered.TemplateBinding.Status = status
	rendered.TemplateBinding.ConflictCount = len(conflicts)
	rendered.TemplateBinding.InvalidPatches = invalidPatches
	return rendered, &WorkflowTemplateBindingPreviewResponse{
		RenderedWorkflow: rendered,
		TemplateBinding:  rendered.TemplateBinding,
		Conflicts:        conflicts,
		InvalidPatches:   invalidPatches,
	}, nil
}

func calculateBindingStatus(workflow *commonmodels.WorkflowV4, targetVersion int) (string, []*WorkflowTemplateBindingConflict, []*commonmodels.InvalidJSONPatch) {
	conflicts := calculateBindingConflicts(workflow, targetVersion)
	invalidPatches := validateWorkflowDeltaPatches(workflow, targetVersion)
	if len(invalidPatches) > 0 {
		return WorkflowTemplateBindingStatusInvalidDelta, conflicts, invalidPatches
	}
	if len(conflicts) > 0 {
		return WorkflowTemplateBindingStatusConflict, conflicts, nil
	}
	latest, err := commonrepo.NewWorkflowV4TemplateVersionColl().GetLatest(workflow.TemplateBinding.TemplateID)
	if err == nil && latest.Version > workflow.TemplateBinding.BaseVersion {
		return WorkflowTemplateBindingStatusTemplateUpdated, nil, nil
	}
	return WorkflowTemplateBindingStatusSynced, nil, nil
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
	templatePatches, err := createJSONPatchOperations(base.Snapshot, target.Snapshot)
	if err != nil {
		return nil
	}
	conflicts := make([]*WorkflowTemplateBindingConflict, 0)
	for _, templatePatch := range templatePatches {
		for _, deltaPatch := range workflow.TemplateBinding.DeltaPatches {
			if jsonPatchPathConflict(templatePatch.Path, deltaPatch.Path) {
				conflicts = append(conflicts, &WorkflowTemplateBindingConflict{Path: deltaPatch.Path})
				break
			}
		}
	}
	return conflicts
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

func getTemplateAndVersionForBinding(templateID string, version int, user string) (*commonmodels.WorkflowV4Template, *commonmodels.WorkflowV4TemplateVersion, error) {
	template, err := commonrepo.NewWorkflowV4TemplateColl().Find(&commonrepo.WorkflowTemplateQueryOption{ID: templateID})
	if err != nil {
		return nil, nil, err
	}
	if _, err := EnsureWorkflowTemplateVersion(template.ID.Hex(), user); err != nil {
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
	if err == mongo.ErrNoDocuments {
		if _, ensureErr := EnsureWorkflowTemplateVersion(templateID, ""); ensureErr != nil {
			return nil, ensureErr
		}
		return commonrepo.NewWorkflowV4TemplateVersionColl().GetLatest(templateID)
	}
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

func validateWorkflowTopologyUnchanged(base, input *commonmodels.WorkflowV4) error {
	if len(base.Stages) != len(input.Stages) {
		return fmt.Errorf("template bound workflow cannot add or delete stages")
	}
	for stageIndex, baseStage := range base.Stages {
		inputStage := input.Stages[stageIndex]
		if baseStage.Name != inputStage.Name {
			return fmt.Errorf("template bound workflow cannot rename or move stage %s", baseStage.Name)
		}
		if len(baseStage.Jobs) != len(inputStage.Jobs) {
			return fmt.Errorf("template bound workflow cannot add or delete jobs in stage %s", baseStage.Name)
		}
		for jobIndex, baseJob := range baseStage.Jobs {
			inputJob := inputStage.Jobs[jobIndex]
			if baseJob.Name != inputJob.Name || baseJob.JobType != inputJob.JobType {
				return fmt.Errorf("template bound workflow cannot rename, move, or change type of job %s", baseJob.Name)
			}
		}
	}
	return nil
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
	if err := validateWorkflowTopologyUnchanged(base, rendered); err != nil {
		return nil, nil, err
	}
	return patches, rendered, nil
}

func validateTemplateBindingPatch(patch *commonmodels.JSONPatchOperation) error {
	if patch == nil {
		return fmt.Errorf("nil json patch operation")
	}
	switch patch.Operation {
	case "add", "remove", "replace":
	default:
		return fmt.Errorf("unsupported template binding patch operation %s", patch.Operation)
	}

	if patch.Path == "/params" || strings.HasPrefix(patch.Path, "/params/") {
		return nil
	}

	pathParts := strings.Split(strings.TrimPrefix(patch.Path, "/"), "/")
	if len(pathParts) >= 5 &&
		pathParts[0] == "stages" &&
		pathParts[2] == "jobs" &&
		pathParts[4] == "spec" {
		return nil
	}

	return fmt.Errorf("template bound workflow patch path %s is not allowed", patch.Path)
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

func jsonPatchPathConflict(a, b string) bool {
	if a == b {
		return true
	}
	a = strings.TrimRight(a, "/")
	b = strings.TrimRight(b, "/")
	return strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}
