package ai

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestInspectionHistoryAlwaysFiltersProjectEnvironmentAndProduction(t *testing.T) {
	for _, production := range []bool{false, true} {
		filter, err := envAnalysisHistoryFilter(EnvAIAnalysisListOption{ProjectName: "p", EnvName: "same-name", Production: production})
		if err != nil || !reflect.DeepEqual(filter, bson.M{"project_name": "p", "env_name": "same-name", "production": production}) {
			t.Fatalf("测试与生产历史必须隔离: %v %v", filter, err)
		}
	}
	for _, opts := range []EnvAIAnalysisListOption{{ProjectName: "p"}, {EnvName: "dev"}, {}} {
		if _, err := envAnalysisHistoryFilter(opts); err == nil {
			t.Fatal("不得退化为跨项目或跨环境查询")
		}
	}
	for _, opts := range []EnvAIAnalysisListOption{{ProjectName: "p", EnvName: "dev", PageNum: -1}, {ProjectName: "p", EnvName: "dev", PageSize: 21}, {ProjectName: "p", EnvName: "dev", PageNum: 10001}} {
		if _, _, err := (&EnvAIAnalysisColl{}).ListByOptions(opts); err == nil {
			t.Fatal("非法分页必须在访问数据库前拒绝")
		}
	}
}
