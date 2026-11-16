package mongodb

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
)

func TestAIReleaseSpecialistConfigPersistence(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	for _, input := range []struct {
		name      string
		documents []bson.D
		want      string
	}{
		{name: "no settings document"},
		{name: "old settings", documents: []bson.D{{{Key: "language", Value: "zh-CN"}}}},
		{name: "configured", documents: []bson.D{{{Key: "ai_release_specialist_config", Value: bson.D{{Key: "llm_integration_id", Value: "selected"}}}}}, want: "selected"},
	} {
		mt.Run(input.name, func(mt *mtest.T) {
			repo := &SystemSettingColl{Collection: mt.Coll}
			mt.AddMockResponses(mtest.CreateCursorResponse(0, mt.DB.Name()+"."+mt.Coll.Name(), mtest.FirstBatch, input.documents...))
			config, err := repo.GetAIReleaseSpecialistConfig(context.Background())
			if err != nil || config == nil || config.LLMIntegrationID != input.want {
				mt.Fatalf("config=%+v err=%v", config, err)
			}
			command := mt.GetAllStartedEvents()[0].Command
			if command.Lookup("filter").Document().Lookup("_id").ObjectID().Hex() != setting.LocalClusterID {
				mt.Fatal("wrong settings document")
			}
		})
	}
	for _, id := range []string{"selected", ""} {
		mt.Run("save "+id, func(mt *mtest.T) {
			repo := &SystemSettingColl{Collection: mt.Coll}
			mt.AddMockResponses(mtest.CreateSuccessResponse(bson.E{Key: "n", Value: 1}, bson.E{Key: "nModified", Value: 1}))
			if err := repo.UpdateAIReleaseSpecialistConfig(context.Background(), &models.AIReleaseSpecialistConfig{LLMIntegrationID: id}); err != nil {
				mt.Fatal(err)
			}
			update := mt.GetAllStartedEvents()[0].Command.Lookup("updates").Array().Index(0).Value().Document()
			if update.Lookup("q").Document().Lookup("_id").ObjectID().Hex() != setting.LocalClusterID || !update.Lookup("upsert").Boolean() {
				mt.Fatal("must upsert system settings")
			}
			set := update.Lookup("u").Document().Lookup("$set").Document()
			elements, err := set.Elements()
			if err != nil || len(elements) != 1 || set.Lookup("ai_release_specialist_config").Document().Lookup("llm_integration_id").StringValue() != id {
				mt.Fatalf("must only update release specialist config: %s", set)
			}
		})
	}
	mt.Run("database errors", func(mt *mtest.T) {
		repo := &SystemSettingColl{Collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 123, Message: "read failed"}))
		if _, err := repo.GetAIReleaseSpecialistConfig(context.Background()); err == nil {
			mt.Fatal("read error ignored")
		}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 123, Message: "write failed"}))
		if err := repo.UpdateAIReleaseSpecialistConfig(context.Background(), &models.AIReleaseSpecialistConfig{}); err == nil {
			mt.Fatal("write error ignored")
		}
	})
}
