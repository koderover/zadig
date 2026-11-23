package mongodb

import (
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestFindReferencingBuild(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("current project and build job", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, mt.DB.Name()+"."+mt.Coll.Name(), mtest.FirstBatch,
			bson.D{{Key: "name", Value: "release"}, {Key: "display_name", Value: "Release"}}))

		workflow, err := (&WorkflowV4Coll{Collection: mt.Coll}).FindReferencingBuild("project-a", "build-a")
		if err != nil || workflow == nil || workflow.Name != "release" || workflow.DisplayName != "Release" {
			mt.Fatalf("expected matching workflow names, got %+v, %v", workflow, err)
		}
		command := mt.GetStartedEvent().Command
		filter := command.Lookup("filter").Document()
		if filter.Lookup("project").StringValue() != "project-a" {
			mt.Fatal("expected project-scoped query")
		}
		job := filter.Lookup("stages", "$elemMatch", "jobs", "$elemMatch").Document()
		if job.Lookup("type").StringValue() != "zadig-build" {
			mt.Fatal("expected references to match within a build job")
		}
		alternatives := job.Lookup("$or").Array()
		for i, path := range []string{
			"spec.service_and_builds.build_name",
			"spec.default_service_and_builds.build_name",
			"spec.service_and_builds_options.build_name",
		} {
			if alternatives.Index(uint(i)).Value().Document().Lookup(path).StringValue() != "build-a" {
				mt.Fatalf("expected exact build name in %s", path)
			}
		}
		if command.Lookup("limit").AsInt64() != 1 || command.Lookup("projection").Document().Lookup("name").AsInt64() != 1 {
			mt.Fatal("expected at most one matching workflow, with name projected")
		}
	})
	mt.Run("no reference", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, mt.DB.Name()+"."+mt.Coll.Name(), mtest.FirstBatch))
		workflow, err := (&WorkflowV4Coll{Collection: mt.Coll}).FindReferencingBuild("project-a", "build-a")
		if workflow != nil || !errors.Is(err, mongo.ErrNoDocuments) {
			mt.Fatalf("expected no reference, got %+v, %v", workflow, err)
		}
	})
}
