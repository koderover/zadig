package mongodb

import (
	"context"
	"fmt"

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

	_, err := c.Indexes().CreateMany(ctx, mod, mongotool.CreateIndexOptions(ctx))
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

func (c *JobInfoColl) GetTestJobsByWorkflow(workflowName string) ([]*models.JobInfo, error) {
	query := bson.M{}
	query["workflow_name"] = workflowName
	query["type"] = config.JobZadigTesting

	resp := make([]*models.JobInfo, 0)

	cursor, err := c.Find(context.Background(), query, options.Find().SetSort(bson.D{{"task_id", -1}}))
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

func (c *JobInfoColl) GetBuildJobsStats(startTime, endTime int64, projectNames []string) (*models.ServiceDeployCountWithStatus, error) {
	query := bson.M{}
	if startTime > 0 && endTime > 0 {
		query["start_time"] = bson.M{"$gte": startTime, "$lt": endTime}
	}
	query["type"] = config.JobZadigBuild

	if len(projectNames) != 0 {
		query["product_name"] = bson.M{"$in": projectNames}
	}

	pipeline := make([]bson.M, 0)
	// find all the deployment jobs
	pipeline = append(pipeline, bson.M{
		"$match": query,
	})

	// group them by project, production and service and get the count of all/failed/success jobs
	pipeline = append(pipeline, bson.M{
		"$group": bson.M{
			"_id":   1,
			"count": bson.M{"$sum": 1},
			"success": bson.M{"$sum": bson.M{
				"$cond": bson.D{
					{"if", bson.D{{"$eq", bson.A{"$status", "passed"}}}},
					{"then", 1},
					{"else", 0},
				},
			}},
			"failed": bson.M{"$sum": bson.M{
				"$cond": bson.D{
					{"if", bson.D{{"$in", bson.A{"$status", bson.A{"timeout", "failed"}}}}},
					{"then", 1},
					{"else", 0},
				},
			}},
		},
	})

	// then do a projection returning all the field back to its position
	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"_id":     0,
			"count":   1,
			"success": 1,
			"failed":  1,
		},
	})

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	ok := cursor.Next(context.TODO())
	if !ok {
		return nil, nil
	}

	result := new(models.ServiceDeployCountWithStatus)
	if err := cursor.Decode(&result); err != nil {
		return nil, err
	}
	return result, err
}

func (c *JobInfoColl) GetDeployJobs(startTime, endTime int64, projectNames []string, productionType config.ProductionType) ([]*models.JobInfo, error) {
	query := bson.M{}
	query["start_time"] = bson.M{"$gte": startTime, "$lt": endTime}
	query["type"] = bson.M{"$in": []string{
		string(config.JobZadigDeploy),
		string(config.JobZadigHelmDeploy),
		string(config.JobZadigHelmChartDeploy),
		string(config.JobDeploy),
		string(config.JobZadigVMDeploy),
		string(config.JobCustomDeploy),
	}}

	switch productionType {
	case config.Production:
		query["production"] = true
	case config.Testing:
		query["production"] = false
	case config.Both:
		break
	default:
		return nil, fmt.Errorf("invlid production type: %s", productionType)
	}

	if len(projectNames) != 0 {
		query["product_name"] = bson.M{"$in": projectNames}
	}

	resp := make([]*models.JobInfo, 0)

	cursor, err := c.Find(context.Background(), query, options.Find())
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)

	return resp, err
}

func (c *JobInfoColl) GetDeployJobsStats(startTime, endTime int64, projectNames []string, productionType config.ProductionType) ([]*models.ServiceDeployCountWithStatus, error) {
	query := bson.M{}
	if startTime > 0 && endTime > 0 {
		query["start_time"] = bson.M{"$gte": startTime, "$lt": endTime}
	}
	query["type"] = bson.M{"$in": []string{
		string(config.JobZadigDeploy),
		string(config.JobZadigHelmDeploy),
		string(config.JobZadigHelmChartDeploy),
		string(config.JobDeploy),
		string(config.JobZadigVMDeploy),
		string(config.JobCustomDeploy),
	}}

	switch productionType {
	case config.Production:
		query["production"] = true
	case config.Testing:
		query["production"] = false
	case config.Both:
		break
	default:
		return nil, fmt.Errorf("invlid production type: %s", productionType)
	}

	if len(projectNames) != 0 {
		query["product_name"] = bson.M{"$in": projectNames}
	}

	pipeline := make([]bson.M, 0)
	// find all the deployment jobs
	pipeline = append(pipeline, bson.M{
		"$match": query,
	})

	// group them by project, production and service and get the count of all/failed/success jobs
	pipeline = append(pipeline, bson.M{
		"$group": bson.M{
			"_id": bson.M{
				"production": "$production",
			},
			"count": bson.M{"$sum": 1},
			"success": bson.M{"$sum": bson.M{
				"$cond": bson.D{
					{"if", bson.D{{"$eq", bson.A{"$status", "passed"}}}},
					{"then", 1},
					{"else", 0},
				},
			}},
			"failed": bson.M{"$sum": bson.M{
				"$cond": bson.D{
					{"if", bson.D{{"$in", bson.A{"$status", bson.A{"timeout", "failed"}}}}},
					{"then", 1},
					{"else", 0},
				},
			}},
		},
	})

	// then do a projection returning all the field back to its position
	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"_id":        0,
			"production": "$_id.production",
			"count":      1,
			"success":    1,
			"failed":     1,
		},
	})

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	result := make([]*models.ServiceDeployCountWithStatus, 0)
	if err := cursor.All(context.TODO(), &result); err != nil {
		return nil, err
	}
	return result, err
}

func (c *JobInfoColl) GetTopDeployedService(startTime, endTime int64, projectNames []string, productionType config.ProductionType, top int) ([]*models.ServiceDeployCountWithStatus, error) {
	pipeline := make([]bson.M, 0)

	query := bson.M{
		"start_time": bson.M{"$gte": startTime, "$lte": endTime},
		"type": bson.M{"$in": []string{
			string(config.JobZadigDeploy),
			string(config.JobZadigHelmDeploy),
			string(config.JobZadigVMDeploy),
		}},
		"service_name": bson.M{"$ne": ""},
	}

	switch productionType {
	case config.Production:
		query["production"] = true
	case config.Testing:
		query["production"] = false
	case config.Both:
		break
	default:
		return nil, fmt.Errorf("invlid production type: %s", productionType)
	}

	if len(projectNames) > 0 {
		query["product_name"] = bson.M{"$in": projectNames}
	}

	// find all the deployment jobs
	pipeline = append(pipeline, bson.M{
		"$match": query,
	})

	// group them by project, production and service and get the count of all/failed/success jobs
	pipeline = append(pipeline, bson.M{
		"$group": bson.M{
			"_id": bson.M{
				"production":   "$production",
				"service_name": "$service_name",
				"product_name": "$product_name",
			},
			"count": bson.M{"$sum": 1},
			"success": bson.M{"$sum": bson.M{
				"$cond": bson.D{
					{"if", bson.D{{"$eq", bson.A{"$status", "passed"}}}},
					{"then", 1},
					{"else", 0},
				},
			}},
			"failed": bson.M{"$sum": bson.M{
				"$cond": bson.D{
					{"if", bson.D{{"$in", bson.A{"$status", bson.A{"timeout", "failed"}}}}},
					{"then", 1},
					{"else", 0},
				},
			}},
		},
	})

	// then do a projection returning all the field back to its position
	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"_id":          0,
			"production":   "$_id.production",
			"service_name": "$_id.service_name",
			"product_name": "$_id.product_name",
			"count":        1,
			"success":      1,
			"failed":       1,
		},
	})

	pipeline = append(pipeline, bson.M{
		"$sort": bson.M{"count": -1},
	})
	pipeline = append(pipeline, bson.M{
		"$limit": top,
	})

	pipeline = append(pipeline, bson.M{
		"$lookup": bson.D{
			{"from", "template_product"},
			{"localField", "product_name"},
			{"foreignField", "product_name"},
			{"as", "template_product_info"},
		},
	})
	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"_id":          0,
			"production":   1,
			"service_name": 1,
			"product_name": 1,
			"count":        1,
			"success":      1,
			"failed":       1,
			"project_name": bson.M{"$first": "$template_product_info.project_name"},
		},
	})

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	result := make([]*models.ServiceDeployCountWithStatus, 0)
	if err := cursor.All(context.TODO(), &result); err != nil {
		return nil, err
	}
	return result, err
}

func (c *JobInfoColl) GetTopDeployFailedService(startTime, endTime int64, projectNames []string, productionType config.ProductionType, top int) ([]*models.ServiceDeployCountWithStatus, error) {
	pipeline := make([]bson.M, 0)

	query := bson.M{
		"start_time": bson.M{"$gte": startTime, "$lte": endTime},
		"type": bson.M{"$in": []string{
			string(config.JobZadigDeploy),
			string(config.JobZadigHelmDeploy),
			string(config.JobZadigVMDeploy),
		}},
		"service_name": bson.M{"$ne": ""},
	}

	switch productionType {
	case config.Production:
		query["production"] = true
	case config.Testing:
		query["production"] = false
	case config.Both:
		break
	default:
		return nil, fmt.Errorf("invlid production type: %s", productionType)
	}

	if len(projectNames) > 0 {
		query["product_name"] = bson.M{"$in": projectNames}
	}

	// find all the deployment jobs
	pipeline = append(pipeline, bson.M{
		"$match": query,
	})

	// group them by project, production and service and get the count of all/failed/success jobs
	pipeline = append(pipeline, bson.M{
		"$group": bson.M{
			"_id": bson.M{
				"production":   "$production",
				"service_name": "$service_name",
				"product_name": "$product_name",
			},
			"count": bson.M{"$sum": bson.M{
				"$cond": bson.D{
					{"if", bson.D{{"$in", bson.A{"$status", bson.A{"timeout", "failed"}}}}},
					{"then", 1},
					{"else", 0},
				},
			}},
		},
	})

	// then do a projection returning all the field back to its position
	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"_id":          0,
			"production":   "$_id.production",
			"service_name": "$_id.service_name",
			"product_name": "$_id.product_name",
			"count":        1,
		},
	})

	pipeline = append(pipeline, bson.M{
		"$sort": bson.M{"count": -1},
	})
	pipeline = append(pipeline, bson.M{
		"$limit": top,
	})
	pipeline = append(pipeline, bson.M{
		"$lookup": bson.D{
			{"from", "template_product"},
			{"localField", "product_name"},
			{"foreignField", "product_name"},
			{"as", "template_product_info"},
		},
	})
	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"_id":          0,
			"production":   1,
			"service_name": 1,
			"product_name": 1,
			"count":        1,
			"project_name": bson.M{"$first": "$template_product_info.project_name"},
		},
	})
	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	result := make([]*models.ServiceDeployCountWithStatus, 0)
	if err := cursor.All(context.TODO(), &result); err != nil {
		return nil, err
	}
	return result, err
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
