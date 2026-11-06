package mongodb

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
)

func TestAIAssistantRuntimeReadsConfigurationOnly(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	id := primitive.NewObjectID()
	for _, input := range []struct {
		name           string
		settings       bson.D
		model          bson.D
		wantModelError bool
	}{
		{name: "未配置时默认关闭并保留九十天"},
		{name: "关闭时不读取失效模型", settings: bson.D{{Key: "enabled", Value: false}, {Key: "llm_integration_id", Value: id.Hex()}}},
		{name: "未选择模型", settings: bson.D{{Key: "enabled", Value: true}}},
		{name: "已删除模型不影响保留策略", settings: bson.D{{Key: "enabled", Value: true}, {Key: "llm_integration_id", Value: id.Hex()}}, wantModelError: true},
		{name: "读取所选模型", settings: bson.D{{Key: "enabled", Value: true}, {Key: "llm_integration_id", Value: id.Hex()}}, model: bson.D{
			{Key: "_id", Value: id}, {Key: "protocol", Value: "openai"}, {Key: "provider_name", Value: "other"},
			{Key: "model", Value: "selected"}, {Key: "token", Value: "model-secret"}, {Key: "base_url", Value: "https://model.test/v1"},
		}},
	} {
		mt.Run(input.name, func(mt *mtest.T) {
			coll := &SystemSettingColl{Collection: mt.Coll}
			docs := []bson.D{}
			if input.settings != nil {
				docs = append(docs, bson.D{{Key: "ai_assistant_config", Value: input.settings}})
			}
			mt.AddMockResponses(mtest.CreateCursorResponse(0, mt.DB.Name()+"."+mt.Coll.Name(), mtest.FirstBatch, docs...))
			wantQueries := 1
			if input.model != nil || input.wantModelError {
				modelsFound := []bson.D{}
				if input.model != nil {
					modelsFound = append(modelsFound, input.model)
				}
				mt.AddMockResponses(mtest.CreateCursorResponse(0, mt.DB.Name()+"."+models.LLMIntegration{}.TableName(), mtest.FirstBatch, modelsFound...))
				wantQueries++
			}
			cfg, err := coll.GetAIAssistantRuntime(context.Background())
			if err != nil || cfg.HistoryRetentionDays != 90 || (cfg.ModelError != "") != input.wantModelError {
				mt.Fatalf("默认值或失效处理错误: %+v %v", cfg, err)
			}
			commands := mt.GetAllStartedEvents()
			if len(commands) != wantQueries {
				mt.Fatalf("不应额外访问模型或代理: %d", len(commands))
			}
			for _, command := range commands {
				if command.CommandName != "find" {
					mt.Fatalf("配置加载不得写入数据库: %s", command.CommandName)
				}
			}
			filter := commands[0].Command.Lookup("filter").Document()
			if filter.Lookup("_id").ObjectID().Hex() != setting.LocalClusterID {
				mt.Fatal("必须读取现有系统配置")
			}
			if input.model != nil {
				if cfg.Token != "model-secret" || cfg.Model != "selected" || cfg.Protocol != "openai" || cfg.Provider != "other" || cfg.BaseURL != "https://model.test/v1" {
					mt.Fatal("未加载所选模型")
				}
				if commands[1].Command.Lookup("filter").Document().Lookup("_id").ObjectID() != id {
					mt.Fatal("未按所选模型 ID 查询")
				}
			} else if cfg.Token != "" {
				mt.Fatal("关闭或失效时不得加载模型凭据")
			}
		})
	}
}

func TestAIAssistantRuntimeRefreshesProxyFromDatabase(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("代理变更下次读取生效且不使用进程缓存", func(mt *mtest.T) {
		coll := &SystemSettingColl{Collection: mt.Coll}
		id := primitive.NewObjectID()
		for _, input := range []struct {
			enabled       bool
			address, want string
		}{
			{true, "first.test", "http://first.test:8080"},
			{true, "second.test", "http://second.test:8080"},
			{false, "second.test", ""},
		} {
			mt.AddMockResponses(
				mtest.CreateCursorResponse(0, mt.DB.Name()+"."+mt.Coll.Name(), mtest.FirstBatch, bson.D{{Key: "ai_assistant_config", Value: bson.D{{Key: "enabled", Value: true}, {Key: "llm_integration_id", Value: id.Hex()}}}}),
				mtest.CreateCursorResponse(0, mt.DB.Name()+"."+models.LLMIntegration{}.TableName(), mtest.FirstBatch, bson.D{{Key: "_id", Value: id}, {Key: "enable_proxy", Value: true}}),
				mtest.CreateCursorResponse(0, mt.DB.Name()+"."+models.Proxy{}.TableName(), mtest.FirstBatch, bson.D{{Key: "type", Value: "http"}, {Key: "address", Value: input.address}, {Key: "port", Value: 8080}, {Key: "enable_repo_proxy", Value: input.enabled}}),
			)
			cfg, err := coll.GetAIAssistantRuntime(context.Background())
			if err != nil || cfg.Proxy != input.want {
				mt.Fatalf("未响应代理变更: %+v %v", cfg, err)
			}
		}
	})
}

func TestAIAssistantRuntimeRejectsInvalidRetentionAndQueryFailure(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("保留期无效", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, mt.DB.Name()+"."+mt.Coll.Name(), mtest.FirstBatch,
			bson.D{{Key: "ai_assistant_config", Value: bson.D{{Key: "history_retention_days", Value: 1}}}}))
		if _, err := (&SystemSettingColl{Collection: mt.Coll}).GetAIAssistantRuntime(context.Background()); err == nil {
			mt.Fatal("非法保留期必须报错")
		}
	})
	mt.Run("系统配置查询失败", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 123, Message: "unavailable"}))
		if _, err := (&SystemSettingColl{Collection: mt.Coll}).GetAIAssistantRuntime(context.Background()); err == nil {
			mt.Fatal("查询失败不得使用默认值")
		}
	})
}

func TestAIAssistantRuntimeProxyFailureBlocksModelWithoutLosingRetention(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("代理查询失败不得静默直连", func(mt *mtest.T) {
		id := primitive.NewObjectID()
		mt.AddMockResponses(
			mtest.CreateCursorResponse(0, mt.DB.Name()+"."+mt.Coll.Name(), mtest.FirstBatch, bson.D{{Key: "ai_assistant_config", Value: bson.D{{Key: "enabled", Value: true}, {Key: "llm_integration_id", Value: id.Hex()}}}}),
			mtest.CreateCursorResponse(0, mt.DB.Name()+"."+models.LLMIntegration{}.TableName(), mtest.FirstBatch, bson.D{{Key: "_id", Value: id}, {Key: "enable_proxy", Value: true}}),
			mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 123, Message: "proxy database credential"}),
		)
		cfg, err := (&SystemSettingColl{Collection: mt.Coll}).GetAIAssistantRuntime(context.Background())
		if err != nil || cfg.HistoryRetentionDays != 90 || cfg.ModelError != "AI 助手代理配置读取失败" || cfg.Proxy != "" {
			mt.Fatalf("代理失败必须阻止模型运行并保留历史策略: %+v %v", cfg, err)
		}
	})
}
