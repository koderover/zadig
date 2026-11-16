package mongodb

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestProductFindSeparatesProductionEnvironments(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	for _, production := range []bool{false, true} {
		name := "测试环境"
		if production {
			name = "生产环境"
		}
		mt.Run(name, func(mt *mtest.T) {
			mt.AddMockResponses(mtest.CreateCursorResponse(0, mt.DB.Name()+"."+mt.Coll.Name(), mtest.FirstBatch,
				bson.D{{Key: "product_name", Value: "p"}, {Key: "env_name", Value: "same-name"}, {Key: "production", Value: production}}))
			product, err := (&ProductColl{Collection: mt.Coll}).Find(&ProductFindOptions{Name: "p", EnvName: "same-name", Production: &production})
			if err != nil || product.Production != production {
				mt.Fatalf("必须读取指定属性的实际环境: %+v %v", product, err)
			}
			filter := mt.GetStartedEvent().Command.Lookup("filter").Document()
			if filter.Lookup("product_name").StringValue() != "p" || filter.Lookup("env_name").StringValue() != "same-name" {
				mt.Fatal("必须保留项目及环境范围")
			}
			if production {
				if !filter.Lookup("production").Boolean() {
					mt.Fatal("生产查询不能匹配同名测试环境")
				}
			} else {
				alternatives := filter.Lookup("$or").Array()
				if alternatives.Index(0).Value().Document().Lookup("production").Document().Lookup("$eq").Boolean() || alternatives.Index(1).Value().Document().Lookup("production").Document().Lookup("$exists").Boolean() {
					mt.Fatal("测试环境只允许 false 或旧记录缺失属性，不能匹配生产环境")
				}
			}
		})
	}
}
