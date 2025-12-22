package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type EnvInfoColl struct {
	*mongo.Collection
	mongo.Session

	coll string
}

func NewEnvInfoColl() *EnvInfoColl {
	name := models.EnvInfo{}.TableName()
	return &EnvInfoColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func NewEnvInfoCollWithSession(session mongo.Session) *EnvInfoColl {
	name := models.EnvInfo{}.TableName()
	return &EnvInfoColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		Session:    session,
		coll:       name,
	}
}

func (c *EnvInfoColl) GetCollectionName() string {
	return c.coll
}

func (c *EnvInfoColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "create_time", Value: 1},
			},
			Options: options.Index().SetUnique(false).SetName("idx_time"),
		},
		{
			Keys: bson.D{
				bson.E{Key: "operation", Value: 1},
				bson.E{Key: "create_time", Value: 1},
			},
			Options: options.Index().SetUnique(false).SetName("idx_operation_time"),
		},
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "operation", Value: 1},
				bson.E{Key: "create_time", Value: 1},
			},
			Options: options.Index().SetUnique(false).SetName("idx_project_operation_time"),
		},
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "env_name", Value: 1},
				bson.E{Key: "operation", Value: 1},
				bson.E{Key: "create_time", Value: 1},
			},
			Options: options.Index().SetUnique(false).SetName("idx_project_env_operation_time"),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())
	return err
}

func (c *EnvInfoColl) Create(ctx *internalhandler.Context, args *models.EnvInfo) error {
	if args == nil {
		return errors.New("configuration management is nil")
	}
	args.CreatTime = time.Now().Unix()
	args.CreatedBy = ctx.GenUserBriefInfo()

	_, err := c.InsertOne(ctx, args)
	return err
}

type ListEnvInfoOption struct {
	PageNum      int
	PageSize     int
	ProjectName  string
	ProjectNames []string
	EnvName      string
	ServiceName  string
	StartTime    int64
	EndTime      int64
	Operation    config.EnvOperation
	Production   *bool
}

func (c *EnvInfoColl) List(ctx context.Context, opt *ListEnvInfoOption) ([]*models.EnvInfo, int64, error) {
	if opt == nil {
		return nil, 0, errors.New("nil ListOption")
	}

	findOption := bson.M{}
	if len(opt.ProjectName) > 0 {
		findOption["project_name"] = opt.ProjectName
	}
	if len(opt.ProjectNames) > 0 {
		findOption["project_name"] = bson.M{"$in": opt.ProjectNames}
	}
	if len(opt.EnvName) > 0 {
		findOption["env_name"] = opt.EnvName
	}
	if len(opt.ServiceName) > 0 {
		findOption["service_name"] = opt.ServiceName
	}
	if opt.Operation != "" {
		findOption["operation"] = opt.Operation
	}
	// 根据 production 参数过滤环境信息
	if opt.Production != nil {
		findOption["production"] = *opt.Production
	}
	findOption["create_time"] = bson.M{
		"$gte": opt.StartTime,
		"$lte": opt.EndTime,
	}

	pipeline := []bson.M{
		{
			"$match": findOption,
		},
		{
			"$sort": bson.M{"create_time": -1}, // order by create_time desc
		},
	}

	if opt.PageNum > 0 && opt.PageSize > 0 {
		pipeline = append(pipeline, bson.M{
			"$facet": bson.M{
				"data": []bson.M{
					{
						"$skip": (opt.PageNum - 1) * opt.PageSize,
					},
					{
						"$limit": opt.PageSize,
					},
				},
				"total": []bson.M{
					{
						"$count": "total",
					},
				},
			},
		})
	} else {
		pipeline = append(pipeline, bson.M{
			"$facet": bson.M{
				"data": []bson.M{
					{"$skip": 0},
				},
				"total": []bson.M{
					{
						"$count": "total",
					},
				},
			},
		})
	}

	result := make([]struct {
		Data  []*models.EnvInfo `bson:"data"`
		Total []struct {
			Total int64 `bson:"total"`
		} `bson:"total"`
	}, 0)

	cursor, err := c.Collection.Aggregate(mongotool.SessionContext(ctx, c.Session), pipeline)
	if err != nil {
		return nil, 0, err
	}

	err = cursor.All(mongotool.SessionContext(ctx, c.Session), &result)
	if err != nil {
		return nil, 0, err
	}

	if len(result) == 0 {
		return nil, 0, nil
	}

	var res []*models.EnvInfo
	if len(result[0].Data) > 0 {
		res = result[0].Data
	}

	total := int64(0)
	if len(result[0].Total) > 0 {
		total = result[0].Total[0].Total
	}

	return res, total, nil
}

type RollbackServiceCount struct {
	Production  bool   `bson:"production"             json:"production"`
	ProjectName string `bson:"project_name,omitempty" json:"project_name,omitempty"`
	ServiceName string `bson:"service_name,omitempty" json:"service_name,omitempty"`
	Count       int    `bson:"count"                  json:"count"`
}

func (c *EnvInfoColl) GetTopRollbackedService(ctx context.Context, startTime, endTime int64, productionType config.ProductionType, projects []string, top int) ([]*RollbackServiceCount, error) {
	// 参数验证
	if startTime > endTime {
		return nil, fmt.Errorf("invalid time range: startTime (%d) should be less than or equal to endTime (%d)", startTime, endTime)
	}
	if top <= 0 {
		return nil, fmt.Errorf("invalid top parameter: top (%d) should be greater than 0", top)
	}

	// 构建查询条件：只查询 rollback 操作
	query := bson.M{
		"operation":   config.EnvOperationRollback,
		"create_time": bson.M{"$gte": startTime, "$lte": endTime},
	}

	// 根据生产环境类型过滤
	switch productionType {
	case config.Production:
		query["production"] = true
	case config.Testing:
		query["production"] = false
	case config.Both:
		break
	default:
		return nil, fmt.Errorf("invalid production type: %s", productionType)
	}

	if len(projects) > 0 {
		query["project_name"] = bson.M{"$in": projects}
	}

	// 构建聚合管道
	pipeline := []bson.M{
		// 匹配 rollback 操作记录
		{"$match": query},
		// 按项目、服务和生产环境类型分组，统计 rollback 次数
		{
			"$group": bson.M{
				"_id": bson.M{
					"production":   "$production",
					"project_name": "$project_name",
					"service_name": "$service_name",
				},
				"count": bson.M{"$sum": 1},
			},
		},
		// 按 rollback 次数降序排序
		{"$sort": bson.M{"count": -1}},
		// 限制返回数量
		{"$limit": top},
		// 投影字段，映射到返回结构
		{
			"$project": bson.M{
				"_id":          0,
				"production":   "$_id.production",
				"project_name": "$_id.project_name",
				"service_name": "$_id.service_name",
				"count":        1,
			},
		},
	}

	// 执行聚合查询
	cursor, err := c.Aggregate(mongotool.SessionContext(ctx, c.Session), pipeline)
	if err != nil {
		return nil, errors.Wrap(err, "failed to aggregate rollback service statistics")
	}
	defer cursor.Close(mongotool.SessionContext(ctx, c.Session))

	// 解析结果
	result := make([]*RollbackServiceCount, 0)
	if err := cursor.All(mongotool.SessionContext(ctx, c.Session), &result); err != nil {
		return nil, errors.Wrap(err, "failed to decode rollback service statistics")
	}

	return result, nil
}
