package mongodb

import (
	"context"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type JobInfoColl struct {
	*mongo.Collection

	coll string
}

func NewJobInfoColl() *JobInfoColl {
	name := models.JobInfo{}.TableName()
	return &JobInfoColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *JobInfoColl) GetCollectionName() string {
	return c.coll
}

func (c *JobInfoColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "product_name", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "start_time", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "type", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)
	return err
}

func (c *JobInfoColl) Create(ctx context.Context, args *models.JobInfo) error {
	if args == nil {
		return errors.New("configuration management is nil")
	}

	_, err := c.InsertOne(ctx, args)
	return err
}

func (c *JobInfoColl) GetProductionDeployJobs(startTime, endTime int64, projectName string) ([]*models.JobInfo, error) {
	query := bson.M{}
	query["start_time"] = bson.M{"$gte": startTime, "$lt": endTime}
	query["production"] = true
	// TODO: currently the only job type with production env update function is zadig-deploy
	// if we added production update for helm-deploy type job, we need to update this query
	query["type"] = config.JobZadigDeploy
	if len(projectName) != 0 {
		query["product_name"] = projectName
	}

	resp := make([]*models.JobInfo, 0)

	cursor, err := c.Find(context.Background(), query, options.Find())
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)

	return resp, err
}

func (c *JobInfoColl) GetTestJobs(startTime, endTime int64, projectName string) ([]*models.JobInfo, error) {
	query := bson.M{}
	query["start_time"] = bson.M{"$gte": startTime, "$lt": endTime}
	query["type"] = config.JobZadigTesting

	if len(projectName) != 0 {
		query["product_name"] = projectName
	}

	resp := make([]*models.JobInfo, 0)

	cursor, err := c.Find(context.Background(), query, options.Find())
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)

	return resp, err
}
