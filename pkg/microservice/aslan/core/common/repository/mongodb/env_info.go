package mongodb

import (
	"context"
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

	_, err := c.Indexes().CreateMany(ctx, mod)
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
				"data": []bson.M{},
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
