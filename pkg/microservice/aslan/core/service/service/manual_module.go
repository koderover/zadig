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

package service

import (
	"context"
	"fmt"
	"regexp"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/setting"
)

// moduleImagePlaceholderRegex matches `$<name>-image$` anywhere in a string
// and captures the name. Mirrors moduleImageRegex in kube/parse.go — kept in
// a third copy here to avoid an import cycle (service/service → kube has
// other forward deps).
//
// TODO: consolidate the three copies (kube/parse.go, util/service.go, here)
// once we have a leaf package both can import safely.
var moduleImagePlaceholderRegex = regexp.MustCompile(`\$([A-Za-z0-9_-]+)-image\$`)

// ManualModuleCreateReq is the API payload for declaring a new manual module.
// Manual modules attach to a K8s YAML service template and are version-
// agnostic — they remain visible across every revision of the service.
type ManualModuleCreateReq struct {
	ServiceName string `json:"service_name"`
	Name        string `json:"name"`
	Image       string `json:"image"`
	ImageName   string `json:"image_name,omitempty"`
}

// ManualModuleUpdateReq is the payload for editing an existing manual module.
// Name is immutable — change name by deleting and recreating.
type ManualModuleUpdateReq struct {
	Image     string `json:"image"`
	ImageName string `json:"image_name,omitempty"`
}

// ListManualServiceModules returns every manual module declared for one
// service. Returns an empty slice (not nil) when there are none.
func ListManualServiceModules(projectName, serviceName string, production bool, log *zap.SugaredLogger) ([]*commonmodels.ServiceModule, error) {
	if projectName == "" || serviceName == "" {
		return nil, fmt.Errorf("projectName and serviceName are required")
	}
	coll := pickModuleColl(production)
	records, err := coll.ListManual(context.Background(), projectName, serviceName)
	if err != nil {
		log.Errorf("failed to list manual modules for %s/%s: %s", projectName, serviceName, err)
		return nil, fmt.Errorf("failed to list manual modules: %s", err)
	}
	if records == nil {
		records = []*commonmodels.ServiceModule{}
	}
	return records, nil
}

// ListAutoServiceModules returns the auto-discovered modules for the current
// service revision so callers can access the record ids needed by the delete
// API.
func ListAutoServiceModules(projectName, serviceName string, production bool, log *zap.SugaredLogger) ([]*commonmodels.ServiceModule, error) {
	if projectName == "" || serviceName == "" {
		return nil, fmt.Errorf("projectName and serviceName are required")
	}
	svc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName:   projectName,
		ServiceName:   serviceName,
		ExcludeStatus: setting.ProductStatusDeleting,
	})
	if production {
		svc, err = commonrepo.NewProductionServiceColl().Find(&commonrepo.ServiceFindOption{
			ProductName:   projectName,
			ServiceName:   serviceName,
			ExcludeStatus: setting.ProductStatusDeleting,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find service %s/%s: %s", projectName, serviceName, err)
	}
	records, err := repository.ListAutoServiceModules(context.Background(), projectName, serviceName, production, svc.Revision)
	if err != nil {
		log.Errorf("failed to list auto modules for %s/%s rev %d: %s", projectName, serviceName, svc.Revision, err)
		return nil, fmt.Errorf("failed to list auto modules: %s", err)
	}
	if records == nil {
		records = []*commonmodels.ServiceModule{}
	}
	return records, nil
}

// CreateManualServiceModule declares a new manual module. Rejects:
//   - empty name or image (service-layer guard mirroring the UI required flag)
//   - service template that does not exist
//   - a duplicate Name within the same (project, service)
//
// ImageName defaults to Name when empty (also enforced at the storage layer
// as belt-and-suspenders).
func CreateManualServiceModule(args *ManualModuleCreateReq, projectName string, production bool, log *zap.SugaredLogger) (*commonmodels.ServiceModule, error) {
	if args == nil {
		return nil, fmt.Errorf("missing request body")
	}
	if projectName == "" || args.ServiceName == "" {
		return nil, fmt.Errorf("projectName and service_name are required")
	}
	if args.Name == "" {
		return nil, fmt.Errorf("module name is required")
	}
	if args.Image == "" {
		return nil, fmt.Errorf("module image is required")
	}
	if !isValidModuleName(args.Name) {
		return nil, fmt.Errorf("module name must match K8s container naming rules ([A-Za-z0-9_-]+)")
	}

	if err := ensureServiceExists(projectName, args.ServiceName, production); err != nil {
		return nil, err
	}

	coll := pickModuleColl(production)
	existing, err := coll.ListManual(context.Background(), projectName, args.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to look up existing manual modules: %s", err)
	}
	for _, m := range existing {
		if m.Name == args.Name {
			return nil, fmt.Errorf("manual module %q already exists for service %s", args.Name, args.ServiceName)
		}
	}

	record := &commonmodels.ServiceModule{
		ProjectName: projectName,
		ServiceName: args.ServiceName,
		Name:        args.Name,
		Image:       args.Image,
		ImageName:   args.ImageName,
	}
	if err := coll.CreateManual(context.Background(), record); err != nil {
		log.Errorf("failed to create manual module %s/%s/%s: %s", projectName, args.ServiceName, args.Name, err)
		return nil, fmt.Errorf("failed to create manual module: %s", err)
	}
	return record, nil
}

// UpdateManualServiceModule replaces Image and ImageName on an existing manual
// record. Name is not editable here — delete + recreate to rename, so the
// $<name>-image$ placeholder references stay consistent.
//
// Both Image and ImageName are required at the API layer; the convenience
// fallback only applies on create.
func UpdateManualServiceModule(id string, production bool, args *ManualModuleUpdateReq, log *zap.SugaredLogger) error {
	if args == nil {
		return fmt.Errorf("missing request body")
	}
	if args.Image == "" {
		return fmt.Errorf("module image is required")
	}
	if args.ImageName == "" {
		return fmt.Errorf("module image_name is required")
	}
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid module id: %s", err)
	}
	if err := pickModuleColl(production).UpdateManual(context.Background(), objectID, args.Image, args.ImageName); err != nil {
		log.Errorf("failed to update manual module %s: %s", id, err)
		return fmt.Errorf("failed to update manual module: %s", err)
	}
	return nil
}

// DeleteManualServiceModule removes a single manual module by id. The
// $<name>-image$ placeholder in any service YAML referring to this module
// will start failing to resolve on next deploy — surfaced as a warning by
// the save-time validator.
func DeleteManualServiceModule(id string, production bool, log *zap.SugaredLogger) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid module id: %s", err)
	}
	if err := pickModuleColl(production).DeleteByID(context.Background(), objectID); err != nil {
		log.Errorf("failed to delete manual module %s: %s", id, err)
		return fmt.Errorf("failed to delete manual module: %s", err)
	}
	return nil
}

// DeleteAutoServiceModule removes one auto-discovered module by id. Manual
// modules are not affected.
func DeleteAutoServiceModule(id string, production bool, log *zap.SugaredLogger) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid module id: %s", err)
	}
	if err := repository.DeleteAutoServiceModuleByID(context.Background(), production, objectID); err != nil {
		log.Errorf("failed to delete auto module %s: %s", id, err)
		return fmt.Errorf("failed to delete auto module: %s", err)
	}
	return nil
}

// ResolveServiceModules is a convenience wrapper around the merge function
// that handler endpoints can call when they need the unified module list
// (auto + manual) plus the conflict report.
func ResolveServiceModules(projectName, serviceName string, production bool, revision int64) ([]*commonmodels.Container, []repository.ModuleConflict, error) {
	return repository.ResolveServiceModules(context.Background(), projectName, serviceName, production, revision)
}

// ValidatePlaceholderResolution scans the rendered YAML for $<name>-image$
// placeholders and returns the names that don't match any known module
// (parsed containers ∪ manual modules). The caller decides what to do —
// callers in this package log + include in the response, never reject the
// save (per the design rule that git-imported templates must not be
// blocked).
func ValidatePlaceholderResolution(svc *commonmodels.Service, production bool) []string {
	if svc == nil || svc.Type != setting.K8SDeployType {
		return nil
	}
	knownNames := make(map[string]struct{}, len(svc.Containers))
	for _, c := range svc.Containers {
		if c != nil && c.Name != "" {
			knownNames[c.Name] = struct{}{}
		}
	}
	manuals, err := pickModuleColl(production).ListManual(context.Background(), svc.ProductName, svc.ServiceName)
	if err == nil {
		for _, m := range manuals {
			knownNames[m.Name] = struct{}{}
		}
	}

	placeholders := extractPlaceholderNames(svc.Yaml)
	if len(placeholders) == 0 {
		// Also scan the rendered yaml if Yaml itself doesn't contain
		// placeholders (variable substitution may inject them).
		placeholders = extractPlaceholderNames(svc.RenderedYaml)
	}
	unresolved := make([]string, 0)
	seen := make(map[string]struct{})
	for _, name := range placeholders {
		if _, ok := knownNames[name]; ok {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		unresolved = append(unresolved, name)
	}
	return unresolved
}

func extractPlaceholderNames(yaml string) []string {
	if yaml == "" {
		return nil
	}
	matches := moduleImagePlaceholderRegex.FindAllStringSubmatch(yaml, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) >= 2 {
			out = append(out, m[1])
		}
	}
	return out
}

func pickModuleColl(production bool) *commonrepo.ServiceModuleColl {
	if production {
		return commonrepo.NewProductionServiceModuleColl()
	}
	return commonrepo.NewServiceModuleColl()
}

func ensureServiceExists(projectName, serviceName string, production bool) error {
	opt := &commonrepo.ServiceFindOption{
		ProductName:   projectName,
		ServiceName:   serviceName,
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	var err error
	if production {
		_, err = commonrepo.NewProductionServiceColl().Find(opt)
	} else {
		_, err = commonrepo.NewServiceColl().Find(opt)
	}
	if err != nil {
		return fmt.Errorf("service %q not found in project %q: %s", serviceName, projectName, err)
	}
	return nil
}

// isValidModuleName accepts the same character set as parseContainer accepts
// for K8s container names (the broader [A-Za-z0-9_-]+ form, matching what
// the moduleImageRegex in kube/parse.go expects). The regex anchors the
// whole string.
func isValidModuleName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}
