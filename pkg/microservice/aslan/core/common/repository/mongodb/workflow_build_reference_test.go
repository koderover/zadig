package mongodb

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestListReferencingBuild(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("all matching workflows in the current project", func(mt *mtest.T) {
		namespace := mt.DB.Name() + "." + mt.Coll.Name()
		mt.AddMockResponses(
			mtest.CreateCursorResponse(1, namespace, mtest.FirstBatch,
				bson.D{{Key: "name", Value: "nightly"}}),
			mtest.CreateCursorResponse(0, namespace, mtest.NextBatch,
				bson.D{{Key: "name", Value: "release"}, {Key: "display_name", Value: "Release"}}),
		)

		workflows, err := (&WorkflowV4Coll{Collection: mt.Coll}).ListReferencingBuild("project-a", "build-a")
		if err != nil || len(workflows) != 2 || workflows[0].Name != "nightly" ||
			workflows[1].Name != "release" || workflows[1].DisplayName != "Release" {
			mt.Fatalf("expected both matching workflow names, got %+v, %v", workflows, err)
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
		if _, err := command.LookupErr("limit"); err == nil {
			mt.Fatal("expected all matching workflows without a limit")
		}
		projection := command.Lookup("projection").Document()
		if projection.Lookup("name").AsInt64() != 1 || projection.Lookup("display_name").AsInt64() != 1 {
			mt.Fatal("expected both name fields to be projected")
		}
		if command.Lookup("sort").Document().Lookup("name").AsInt64() != 1 {
			mt.Fatal("expected workflows in name order")
		}
	})
	mt.Run("no reference", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, mt.DB.Name()+"."+mt.Coll.Name(), mtest.FirstBatch))
		workflows, err := (&WorkflowV4Coll{Collection: mt.Coll}).ListReferencingBuild("project-a", "build-a")
		if len(workflows) != 0 || err != nil {
			mt.Fatalf("expected no reference, got %+v, %v", workflows, err)
		}
	})
}
