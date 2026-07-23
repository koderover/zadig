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

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/setting"
)

// ServiceModule is a first-class record of a deployable unit ("module" /
// "container") attached to a service template. Modules can originate from:
//
//   - Auto-discovery: parsed out of a service template's KubeYamls for the
//     workload kinds we recognize (Deployment, StatefulSet, Job, CronJob,
//     CloneSet). Auto records are bound to a specific service revision via
//     RevisionBound and are re-derived every time SetCurrentContainerImages
//     re-parses YAML.
//
//   - Manual declaration: added through the manual-module API by users for
//     workload kinds we cannot parse (CRDs, DaemonSets, Argo Rollouts, etc.).
//     Manual records carry RevisionBound = 0 — they are version-agnostic and
//     load for every revision of the service.
//
// Read-merge rule (see ResolveServiceModules): records are unioned by Name with
// time-of-creation precedence ("first-come-first-served" by CreateTime).
// Conflicts are recorded and surfaced to the caller so the UI can warn users.
type ServiceModule struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	ProjectName string             `bson:"project_name"              json:"project_name"`
	ServiceName string             `bson:"service_name"              json:"service_name"`

	// IsManual: true if added explicitly by the user via the manual-module API.
	// Manual records MUST have RevisionBound = 0.
	IsManual bool `bson:"is_manual"                 json:"is_manual"`

	// RevisionBound: the service revision this record belongs to.
	//   0   -> manual / version-agnostic, loads for every revision
	//   >0  -> auto-discovered, scoped to that revision only
	RevisionBound int64 `bson:"revision_bound"            json:"revision_bound"`

	// Name is the module identifier. For auto records it equals the parsed
	// container's Name; for manual records it is the user-supplied name that
	// $<name>-image$ placeholders in YAML resolve against.
	Name string `bson:"name"                      json:"name"`

	// Type carries the legacy Container.Type distinction (normal vs init
	// container). No K8s deployment logic in this codebase branches on this
	// field — it is preserved for OpenAPI parity. Manual records default to
	// ContainerTypeNormal ("").
	Type setting.ContainerType `bson:"type,omitempty"            json:"type,omitempty"`

	// Image is the resolved image URI. For auto records it is whatever the
	// parsed YAML had; for manual records it is what the user typed. Either
	// may be empty at creation time (an unbuilt module).
	Image string `bson:"image,omitempty"           json:"image,omitempty"`

	// ImageName is the registry-friendly name used for build target paths and
	// image webhook matching. Required for manual records (enforced at the
	// service layer); for auto records it is derived from Image via
	// ExtractImageName, falling back to Name.
	ImageName string         `bson:"image_name,omitempty"      json:"image_name,omitempty"`
	ImagePath *ImagePathSpec `bson:"image_path,omitempty"      json:"image_path,omitempty"`

	// Ignored marks an auto-discovered module as hidden for this revision.
	// It is used as a tombstone when users delete an auto module, so later YAML
	// re-parses (for example saving variables) don't silently add it back.
	Ignored bool `bson:"ignored,omitempty"         json:"ignored,omitempty"`

	CreateTime int64 `bson:"create_time"               json:"create_time"`
	UpdateTime int64 `bson:"update_time,omitempty"     json:"update_time,omitempty"`
}

func (ServiceModule) TableName() string {
	return "service_module"
}
