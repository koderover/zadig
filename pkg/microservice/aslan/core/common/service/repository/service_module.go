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

package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

// ModuleConflict reports that two records share a Name. The Winner is the
// record currently in effect under the precedence rule (see
// ResolveServiceModules); Shadowed lists the records that were displaced.
// Callers (typically API handlers) should surface these to the UI so users
// can disambiguate — e.g., "module 'api' has both a manual declaration and
// an auto-discovered entry; the manual one is winning."
type ModuleConflict struct {
	Name     string
	Winner   *models.ServiceModule
	Shadowed []*models.ServiceModule
}

// ServiceModuleSnapshot holds modules preloaded for one request.
type ServiceModuleSnapshot struct {
	Manual      map[string][]*models.Container
	Resolved    map[string]map[int64][]*models.Container
	RecordCount int
}

// LoadServiceModuleSnapshot loads all records required for serviceRevisions
// in one MongoDB find and prepares the same merge results as
// ResolveServiceModules.
func LoadServiceModuleSnapshot(ctx context.Context, projectName string, production bool, serviceRevisions map[string][]int64) (*ServiceModuleSnapshot, error) {
	records, err := pickServiceModuleColl(production).ListByServiceRevisions(ctx, projectName, serviceRevisions)
	if err != nil {
		return nil, err
	}
	return newServiceModuleSnapshot(records, serviceRevisions), nil
}

func newServiceModuleSnapshot(records []*models.ServiceModule, serviceRevisions map[string][]int64) *ServiceModuleSnapshot {
	snapshot := &ServiceModuleSnapshot{
		Manual:      make(map[string][]*models.Container),
		Resolved:    make(map[string]map[int64][]*models.Container),
		RecordCount: len(records),
	}
	recordsByService := make(map[string][]*models.ServiceModule)
	for _, record := range records {
		if record == nil {
			continue
		}
		recordsByService[record.ServiceName] = append(recordsByService[record.ServiceName], record)
		if record.IsManual {
			snapshot.Manual[record.ServiceName] = append(snapshot.Manual[record.ServiceName], &models.Container{
				Name: record.Name, Type: record.Type, Image: record.Image, ImageName: record.ImageName, ImagePath: record.ImagePath,
			})
		}
	}

	for serviceName, revisions := range serviceRevisions {
		snapshot.Resolved[serviceName] = make(map[int64][]*models.Container)
		for _, revision := range revisions {
			selected := make([]*models.ServiceModule, 0)
			for _, record := range recordsByService[serviceName] {
				// ListByServiceRevision historically excludes ignored records for
				// both branches, while the standalone manual listing does not.
				if record.Ignored {
					continue
				}
				if record.IsManual || record.RevisionBound == revision {
					selected = append(selected, record)
				}
			}
			snapshot.Resolved[serviceName][revision] = mergeServiceModules(selected)
		}
	}
	return snapshot
}

// ResolveServiceModules returns the merged module list for one (project,
// service, revision) plus any name conflicts. Production picks the right
// underlying collection (service_module vs production_service_module).
//
// Merge rule: time-of-creation precedence ("first-come-first-served"). For
// every Name in the union of (manual records, auto records bound to
// `revision`), the record with the smallest CreateTime wins; later records
// with the same Name are reported as shadowed. Ties (same CreateTime) are
// broken by ObjectID order — deterministic, but "shouldn't happen in
// practice."
//
// The returned []*models.Container is a drop-in replacement for the legacy
// Service.Containers field, so callers migrating off the field only need to
// swap their data source.
func ResolveServiceModules(ctx context.Context, projectName, serviceName string, production bool, revision int64) ([]*models.Container, []ModuleConflict, error) {
	coll := pickServiceModuleColl(production)
	records, err := coll.ListByServiceRevision(ctx, projectName, serviceName, revision)
	if err != nil {
		return nil, nil, err
	}
	return mergeServiceModules(records), conflictsFromMerge(records), nil
}

// SyncAutoServiceModules upserts the parsed containers from svc.Containers
// into the service_module collection as auto records bound to svc.Revision.
// Idempotent: replaces the existing auto records for (service, revision)
// atomically.
//
// Called from Create / UpdateServiceContainers in this package — see those
// functions for the integration points. External callers (e.g. backfill in
// UpgradeAssistant) can call this directly.
//
// Manual records for the service are untouched.
func SyncAutoServiceModules(ctx context.Context, svc *models.Service, production bool) error {
	if svc == nil || svc.ServiceName == "" || svc.ProductName == "" || svc.Revision == 0 {
		return nil
	}
	coll := pickServiceModuleColl(production)
	records := containersToAutoRecords(svc.ProductName, svc.ServiceName, svc.Revision, svc.Containers)
	ignoredRecords, err := coll.ListIgnoredAutoByRevision(ctx, svc.ProductName, svc.ServiceName, svc.Revision)
	if err != nil {
		return err
	}
	records = filterIgnoredAutoRecords(records, ignoredRecords)
	return coll.ReplaceAutoForRevision(ctx, svc.ProductName, svc.ServiceName, svc.Revision, records)
}

// ListManualServiceModules returns just the manual records for a service,
// mapped to Container shape. Render paths use this to inject user-declared
// modules into the substitution pipeline alongside the in-memory parsed
// containers (which only cover built-in workload kinds).
func ListManualServiceModules(ctx context.Context, projectName, serviceName string, production bool) ([]*models.Container, error) {
	if projectName == "" || serviceName == "" {
		return nil, nil
	}
	records, err := pickServiceModuleColl(production).ListManual(ctx, projectName, serviceName)
	if err != nil {
		return nil, err
	}
	out := make([]*models.Container, 0, len(records))
	for _, r := range records {
		out = append(out, &models.Container{
			Name:      r.Name,
			Type:      r.Type,
			Image:     r.Image,
			ImageName: r.ImageName,
		})
	}
	return out, nil
}

// ListAutoServiceModules returns the auto-discovered records for one service
// revision in their raw ServiceModule shape so callers can access record ids
// for follow-up operations like delete-by-id.
func ListAutoServiceModules(ctx context.Context, projectName, serviceName string, production bool, revision int64) ([]*models.ServiceModule, error) {
	if projectName == "" || serviceName == "" || revision == 0 {
		return nil, nil
	}
	return pickServiceModuleColl(production).ListAutoByRevision(ctx, projectName, serviceName, revision)
}

// CloneAutoServiceModulesForRevision carries visible auto-discovered modules
// forward for a metadata-only service revision. Manual modules are not copied
// because they are revision-independent.
func CloneAutoServiceModulesForRevision(ctx context.Context, projectName, serviceName string, production bool, sourceRevision, targetRevision int64) error {
	if projectName == "" || serviceName == "" || sourceRevision == 0 || targetRevision == 0 || sourceRevision == targetRevision {
		return nil
	}
	records, err := ListAutoServiceModules(ctx, projectName, serviceName, production, sourceRevision)
	if err != nil {
		return err
	}
	clones := make([]*models.ServiceModule, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		clones = append(clones, &models.ServiceModule{
			Name:       record.Name,
			Type:       record.Type,
			Image:      record.Image,
			ImageName:  record.ImageName,
			ImagePath:  record.ImagePath,
			CreateTime: record.CreateTime,
		})
	}
	return pickServiceModuleColl(production).ReplaceAutoForRevision(ctx, projectName, serviceName, targetRevision, clones)
}

// DeleteAutoServiceModulesForRevision drops every auto record for one
// (service, revision). Used by Delete in this package when a specific
// revision is reaped. Manual records are untouched.
func DeleteAutoServiceModulesForRevision(ctx context.Context, projectName, serviceName string, production bool, revision int64) error {
	if projectName == "" || serviceName == "" || revision == 0 {
		return nil
	}
	return pickServiceModuleColl(production).DeleteAutoByRevision(ctx, projectName, serviceName, revision)
}

// DeleteAllServiceModulesForService cascades: removes every record (manual
// and auto, all revisions) for one service. Called when a service template
// is fully deleted (DeleteServiceTemplate).
func DeleteAllServiceModulesForService(ctx context.Context, projectName, serviceName string, production bool) error {
	if projectName == "" || serviceName == "" {
		return nil
	}
	return pickServiceModuleColl(production).DeleteByService(ctx, projectName, serviceName)
}

// DeleteAutoServiceModuleByID removes one auto-discovered module by ObjectID.
// Manual modules are intentionally protected by the storage-layer is_manual
// filter.
func DeleteAutoServiceModuleByID(ctx context.Context, production bool, id primitive.ObjectID) error {
	return pickServiceModuleColl(production).DeleteAutoByID(ctx, id)
}

// RestoreAutoServiceModulesForRevision clears ignored tombstones for one
// revision. Callers should run SyncAutoServiceModules afterwards to re-parse
// the current YAML and drop records that no longer exist in the template.
func RestoreAutoServiceModulesForRevision(ctx context.Context, projectName, serviceName string, production bool, revision int64) error {
	if projectName == "" || serviceName == "" || revision == 0 {
		return nil
	}
	return pickServiceModuleColl(production).RestoreIgnoredAutoByRevision(ctx, projectName, serviceName, revision)
}

func pickServiceModuleColl(production bool) *mongodb.ServiceModuleColl {
	if production {
		return mongodb.NewProductionServiceModuleColl()
	}
	return mongodb.NewServiceModuleColl()
}

func containersToAutoRecords(projectName, serviceName string, revision int64, containers []*models.Container) []*models.ServiceModule {
	records := make([]*models.ServiceModule, 0, len(containers))
	for _, c := range containers {
		if c == nil || c.Name == "" || c.Name == "<nil>" {
			continue
		}
		records = append(records, &models.ServiceModule{
			ProjectName:   projectName,
			ServiceName:   serviceName,
			IsManual:      false,
			RevisionBound: revision,
			Name:          c.Name,
			Type:          c.Type,
			Image:         c.Image,
			ImageName:     normalizeImageName(c.ImageName, c.Name),
			ImagePath:     c.ImagePath,
		})
	}
	return records
}

func filterIgnoredAutoRecords(records []*models.ServiceModule, ignoredRecords []*models.ServiceModule) []*models.ServiceModule {
	if len(records) == 0 || len(ignoredRecords) == 0 {
		return records
	}
	ignoredNames := make(map[string]struct{}, len(ignoredRecords))
	for _, r := range ignoredRecords {
		if r != nil && r.Name != "" {
			ignoredNames[r.Name] = struct{}{}
		}
	}
	filtered := make([]*models.ServiceModule, 0, len(records))
	for _, r := range records {
		if r == nil {
			continue
		}
		if _, ok := ignoredNames[r.Name]; ok {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// normalizeImageName mirrors the legacy GetImageNameFromContainerInfo
// fallback (empty ImageName falls back to Name) at write time. Stored data
// is therefore always non-empty for ImageName, so read paths don't need to
// keep the fallback around. Applies to both auto and manual records.
func normalizeImageName(imageName, name string) string {
	if imageName != "" {
		return imageName
	}
	return name
}

// mergeServiceModules applies the first-come-first-served rule and returns
// the winning records mapped to Container shape. Input MUST be pre-sorted by
// CreateTime ascending then ObjectID — ListByServiceRevision already does so.
func mergeServiceModules(records []*models.ServiceModule) []*models.Container {
	winners := make([]*models.Container, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, r := range records {
		if _, ok := seen[r.Name]; ok {
			continue
		}
		seen[r.Name] = struct{}{}
		winners = append(winners, &models.Container{
			Name:      r.Name,
			Type:      r.Type,
			Image:     r.Image,
			ImageName: r.ImageName,
			ImagePath: r.ImagePath,
		})
	}
	return winners
}

// TODO: convenience wrapper ResolveServiceModulesFor(ctx, svc, production)
// that infers project/service/revision from svc — deferred per discussion,
// callers will pass the explicit args until usage volume justifies it.

// conflictsFromMerge walks the same pre-sorted slice and groups shadowed
// records under their winning entry. Returned slice is empty when there are
// no name collisions.
func conflictsFromMerge(records []*models.ServiceModule) []ModuleConflict {
	byName := make(map[string][]*models.ServiceModule, len(records))
	order := make([]string, 0, len(records))
	for _, r := range records {
		if _, ok := byName[r.Name]; !ok {
			order = append(order, r.Name)
		}
		byName[r.Name] = append(byName[r.Name], r)
	}
	conflicts := make([]ModuleConflict, 0)
	for _, name := range order {
		group := byName[name]
		if len(group) < 2 {
			continue
		}
		conflicts = append(conflicts, ModuleConflict{
			Name:     name,
			Winner:   group[0],
			Shadowed: group[1:],
		})
	}
	return conflicts
}
