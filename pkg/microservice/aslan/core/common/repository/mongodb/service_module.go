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

package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

const (
	serviceModuleCollName           = "service_module"
	productionServiceModuleCollName = "production_service_module"
)

// ServiceModuleColl is the storage for first-class module records.
// Production and non-production records live in separate MongoDB collections
// (same shape) — pick the right constructor for the context, mirroring how
// ServiceColl / ProductionServiceColl is structured.
type ServiceModuleColl struct {
	*mongo.Collection
	mongo.Session
	coll string
}

func NewServiceModuleColl() *ServiceModuleColl {
	return newServiceModuleColl(serviceModuleCollName, nil)
}

func NewProductionServiceModuleColl() *ServiceModuleColl {
	return newServiceModuleColl(productionServiceModuleCollName, nil)
}

func NewServiceModuleCollWithSession(session mongo.Session) *ServiceModuleColl {
	return newServiceModuleColl(serviceModuleCollName, session)
}

func NewProductionServiceModuleCollWithSession(session mongo.Session) *ServiceModuleColl {
	return newServiceModuleColl(productionServiceModuleCollName, session)
}

func newServiceModuleColl(name string, session mongo.Session) *ServiceModuleColl {
	return &ServiceModuleColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		Session:    session,
		coll:       name,
	}
}

func (c *ServiceModuleColl) GetCollectionName() string {
	return c.coll
}

func (c *ServiceModuleColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		// Uniqueness: a given slot (manual or per-revision auto) holds at most
		// one record per name. Manual and auto with the same name coexist —
		// the merge layer (ResolveServiceModules) reconciles them.
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "service_name", Value: 1},
				bson.E{Key: "is_manual", Value: 1},
				bson.E{Key: "revision_bound", Value: 1},
				bson.E{Key: "name", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		// Hot path: list-all-modules-for-a-service.
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "service_name", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}
	_, err := c.Indexes().CreateMany(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

// ListByServiceRevision returns the records relevant to a single read:
// every manual record for the service (RevisionBound = 0) plus the auto
// records bound to the supplied revision. The merge logic in
// ResolveServiceModules deduplicates and applies the precedence rule.
func (c *ServiceModuleColl) ListByServiceRevision(ctx context.Context, projectName, serviceName string, revision int64) ([]*models.ServiceModule, error) {
	query := bson.M{
		"project_name": projectName,
		"service_name": serviceName,
		"$or": bson.A{
			bson.M{"is_manual": true},
			bson.M{"is_manual": false, "revision_bound": revision},
		},
	}
	return c.findAll(ctx, query, options.Find().SetSort(bson.D{
		bson.E{Key: "create_time", Value: 1},
		bson.E{Key: "_id", Value: 1},
	}))
}

// ListManual returns every manual record for a service. Used by the manual-
// module CRUD API to list user-declared modules independently of any revision.
func (c *ServiceModuleColl) ListManual(ctx context.Context, projectName, serviceName string) ([]*models.ServiceModule, error) {
	query := bson.M{
		"project_name": projectName,
		"service_name": serviceName,
		"is_manual":    true,
	}
	return c.findAll(ctx, query, options.Find().SetSort(bson.D{
		bson.E{Key: "create_time", Value: 1},
		bson.E{Key: "_id", Value: 1},
	}))
}

// ListAutoByRevision returns just the auto-discovered records for one revision.
// Used by the write side (SetCurrentContainerImages flow) when it needs to
// reconcile parsed-from-YAML records against the existing snapshot.
func (c *ServiceModuleColl) ListAutoByRevision(ctx context.Context, projectName, serviceName string, revision int64) ([]*models.ServiceModule, error) {
	query := bson.M{
		"project_name":   projectName,
		"service_name":   serviceName,
		"is_manual":      false,
		"revision_bound": revision,
	}
	return c.findAll(ctx, query, nil)
}

func (c *ServiceModuleColl) findAll(ctx context.Context, query bson.M, opts *options.FindOptions) ([]*models.ServiceModule, error) {
	cursor, err := c.Collection.Find(mongotool.SessionContext(ctx, c.Session), query, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	resp := make([]*models.ServiceModule, 0)
	if err := cursor.All(ctx, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CreateManual inserts a single user-declared module. The caller is
// responsible for validating that ImageName is non-empty and that no manual
// record with the same name already exists (uniqueness index will also enforce
// this and surface a duplicate-key error).
//
// ImageName is defensively defaulted to Name if empty — the API layer should
// already require it, but persisting an empty value would break read paths
// that no longer carry the legacy GetImageNameFromContainerInfo fallback.
func (c *ServiceModuleColl) CreateManual(ctx context.Context, m *models.ServiceModule) error {
	m.IsManual = true
	m.RevisionBound = 0
	if m.ImageName == "" {
		m.ImageName = m.Name
	}
	now := time.Now().Unix()
	if m.CreateTime == 0 {
		m.CreateTime = now
	}
	m.UpdateTime = now
	_, err := c.Collection.InsertOne(mongotool.SessionContext(ctx, c.Session), m)
	return err
}

// UpdateManual replaces a manual record's mutable fields by ID.
func (c *ServiceModuleColl) UpdateManual(ctx context.Context, id primitive.ObjectID, image, imageName string) error {
	update := bson.M{
		"$set": bson.M{
			"image":       image,
			"image_name":  imageName,
			"update_time": time.Now().Unix(),
		},
	}
	_, err := c.Collection.UpdateOne(
		mongotool.SessionContext(ctx, c.Session),
		bson.M{"_id": id, "is_manual": true},
		update,
	)
	return err
}

// DeleteByID removes a single record by ObjectID. Used for manual-module
// deletion from the API.
func (c *ServiceModuleColl) DeleteByID(ctx context.Context, id primitive.ObjectID) error {
	_, err := c.Collection.DeleteOne(mongotool.SessionContext(ctx, c.Session), bson.M{"_id": id})
	return err
}

// ReplaceAutoForRevision atomically replaces the auto-discovered records for
// one (service, revision). Used by SetCurrentContainerImages on every YAML
// re-parse. Manual records are not touched.
//
// "Atomic" here means within the deletion-and-insertion window: if there is
// no session, a concurrent reader of the same (service, revision) may observe
// an empty auto set for a brief moment. Pass a session via *WithSession to
// guarantee snapshot consistency.
func (c *ServiceModuleColl) ReplaceAutoForRevision(ctx context.Context, projectName, serviceName string, revision int64, records []*models.ServiceModule) error {
	sessCtx := mongotool.SessionContext(ctx, c.Session)

	if _, err := c.Collection.DeleteMany(sessCtx, bson.M{
		"project_name":   projectName,
		"service_name":   serviceName,
		"is_manual":      false,
		"revision_bound": revision,
	}); err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	now := time.Now().Unix()
	docs := make([]interface{}, 0, len(records))
	for _, r := range records {
		r.ProjectName = projectName
		r.ServiceName = serviceName
		r.IsManual = false
		r.RevisionBound = revision
		if r.CreateTime == 0 {
			r.CreateTime = now
		}
		r.UpdateTime = now
		docs = append(docs, r)
	}
	_, err := c.Collection.InsertMany(sessCtx, docs)
	return err
}

// DeleteAutoByRevision drops every auto record for a specific revision. Used
// when a service revision is hard-deleted (rare cleanup path).
func (c *ServiceModuleColl) DeleteAutoByRevision(ctx context.Context, projectName, serviceName string, revision int64) error {
	_, err := c.Collection.DeleteMany(mongotool.SessionContext(ctx, c.Session), bson.M{
		"project_name":   projectName,
		"service_name":   serviceName,
		"is_manual":      false,
		"revision_bound": revision,
	})
	return err
}

// DeleteAutoByID removes one auto-discovered module by ObjectID. Manual
// records with the same ID are intentionally untouched by the is_manual guard.
func (c *ServiceModuleColl) DeleteAutoByID(ctx context.Context, id primitive.ObjectID) error {
	_, err := c.Collection.DeleteOne(mongotool.SessionContext(ctx, c.Session), bson.M{
		"_id":       id,
		"is_manual": false,
	})
	return err
}

// DeleteByService cascades: removes every record (manual and auto, all
// revisions) for one service. Called when a service template is deleted.
func (c *ServiceModuleColl) DeleteByService(ctx context.Context, projectName, serviceName string) error {
	_, err := c.Collection.DeleteMany(mongotool.SessionContext(ctx, c.Session), bson.M{
		"project_name": projectName,
		"service_name": serviceName,
	})
	return err
}
