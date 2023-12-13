package mongodb

import (
	"context"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
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
	query["type"] = bson.M{"$in": []string{
		string(config.JobZadigDeploy),
		string(config.JobZadigHelmDeploy),
		string(config.JobZadigHelmChartDeploy),
		string(config.JobDeploy),
	}}
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

func (c *JobInfoColl) GetBuildJobs(startTime, endTime int64, projectName string) ([]*models.JobInfo, error) {
	query := bson.M{}
	query["start_time"] = bson.M{"$gte": startTime, "$lt": endTime}
	query["type"] = config.JobZadigBuild

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

func (c *JobInfoColl) GetDeployJobs(startTime, endTime int64, projectName string) ([]*models.JobInfo, error) {
	query := bson.M{}
	query["start_time"] = bson.M{"$gte": startTime, "$lt": endTime}
	query["type"] = bson.M{"$in": []string{
		string(config.JobZadigDeploy),
		string(config.JobZadigHelmDeploy),
		string(config.JobZadigHelmChartDeploy),
		string(config.JobDeploy),
	}}

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

type JobInfoCoarseGrainedData struct {
	StartTime   int64             `json:"start_time"`
	EndTime     int64             `json:"end_time"`
	MonthlyStat []*MonthlyJobInfo `json:"monthly_stat"`
}

type MonthlyJobInfo struct {
	Month       int `bson:"month" json:"month"`
	BuildCount  int `bson:"build_count" json:"build_count"`
	DeployCount int `bson:"deploy_count" json:"deploy_count"`
	TestCount   int `bson:"test_count" json:"test_count"`
}

func (c *JobInfoColl) GetJobInfos(startTime, endTime int64, projectName []string) ([]*models.JobInfo, error) {
	query := bson.M{
		"start_time": bson.M{"$gte": startTime, "$lt": endTime},
	}

	if projectName != nil && len(projectName) != 0 {
		query["product_name"] = bson.M{"$in": projectName}
	}

	resp := make([]*models.JobInfo, 0)
	cursor, err := c.Find(context.Background(), query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.Background(), &resp)
	return resp, err
}

type JobBuildTrendInfos struct {
	ProjectName string            `bson:"_id" json:"product_name"`
	Documents   []*models.JobInfo `bson:"documents" json:"documents"`
}

func (c *JobInfoColl) GetJobBuildTrendInfos(startTime, endTime int64, projectName []string) ([]*JobBuildTrendInfos, error) {
	match := bson.M{"$match": bson.M{"start_time": bson.M{"$gte": startTime, "$lt": endTime}}}
	if projectName != nil && len(projectName) != 0 {
		match["$match"].(bson.M)["product_name"] = bson.M{"$in": projectName}
	}

	group := bson.M{
		"$group": bson.M{
			"_id":       "$product_name",
			"documents": bson.M{"$push": "$value"},
		},
	}
	pipeline := []bson.M{match, group}

	cursor, err := c.Find(context.Background(), pipeline)
	if err != nil {
		return nil, err
	}

	resp := make([]*JobBuildTrendInfos, 0)
	err = cursor.All(context.Background(), &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *JobInfoColl) GetBuildTrend(startTime, endTime int64, projectName []string) ([]*models.JobInfo, error) {
	query := bson.M{
		"start_time": bson.M{"$gte": startTime, "$lt": endTime},
		"type":       config.JobZadigBuild,
	}
	if projectName != nil && len(projectName) != 0 {
		query["product_name"] = bson.M{"$in": projectName}
	}

	resp := make([]*models.JobInfo, 0)
	opts := options.Find().SetSort(bson.D{{"start_time", -1}})
	cursor, err := c.Find(context.Background(), query, opts)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.Background(), &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *JobInfoColl) GetAllProjectNameByTypeName(startTime, endTime int64, typeName string) ([]string, error) {
	query := bson.M{}
	if startTime != 0 && endTime != 0 {
		query["start_time"] = bson.M{"$gte": startTime, "$lt": endTime}
	}
	if typeName != "" {
		query["type"] = typeName
	}

	distinct, err := c.Distinct(context.Background(), "product_name", query)
	if err != nil {
		return nil, err
	}

	resp := make([]string, 0)
	for _, v := range distinct {
		if name, ok := v.(string); ok {
			if name == "" {
				continue
			}
			resp = append(resp, name)
		}
	}
	return resp, nil
}

func (c *JobInfoColl) GetTestTrend(startTime, endTime int64, projectName []string) ([]*models.JobInfo, error) {
	query := bson.M{
		"start_time":   bson.M{"$gte": startTime, "$lt": endTime},
		"product_name": bson.M{"$in": projectName},
		"type":         config.JobZadigTesting,
	}

	resp := make([]*models.JobInfo, 0)
	opts := options.Find().SetSort(bson.D{{"start_time", -1}})
	cursor, err := c.Find(context.Background(), query, opts)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.Background(), &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *JobInfoColl) GetDeployTrend(startTime, endTime int64, projectName []string) ([]*models.JobInfo, error) {
	query := bson.M{
		"start_time":   bson.M{"$gte": startTime, "$lt": endTime},
		"product_name": bson.M{"$in": projectName},
		"type":         config.JobZadigDeploy,
	}

	resp := make([]*models.JobInfo, 0)
	opts := options.Find().SetSort(bson.D{{"start_time", -1}})
	cursor, err := c.Find(context.Background(), query, opts)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.Background(), &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
