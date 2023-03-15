package mongodb

import (
	"context"
	"time"

	"github.com/koderover/zadig/pkg/util"
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

type ProductionDeployJobStats struct {
	Total        int64
	SuccessCount int64
	DailyStat    []*DailyStat
}

type DailyStat struct {
	Date         string
	Total        int64
	SuccessCount int64
	FailCount    int64
}

func (c *JobInfoColl) GetProductionDeployJobs(startTime, endTime int64, projectName string) (*ProductionDeployJobStats, error) {
	resp := new(ProductionDeployJobStats)
	// first get all the production deploy jobs count
	productionAllQuery := bson.M{}
	productionAllQuery["create_time"] = bson.M{"$gte": startTime, "$lt": endTime}
	productionAllQuery["production"] = true
	// TODO: currently production deployment is only available for zadig deploy
	productionAllQuery["type"] = config.JobZadigDeploy
	if len(projectName) != 0 {
		productionAllQuery["product_name"] = bson.M{"$in": projectName}
	}

	productionDeployCount, err := c.CountDocuments(context.TODO(), productionAllQuery, &options.CountOptions{})
	if err != nil {
		return nil, err
	}

	resp.Total = productionDeployCount

	// then we get all passed production
	productionDeployPassedQuery := bson.M{}
	productionDeployPassedQuery["create_time"] = bson.M{"$gte": startTime, "$lt": endTime}
	productionDeployPassedQuery["production"] = true
	productionDeployPassedQuery["status"] = config.StatusPassed
	// TODO: currently production deployment is only available for zadig deploy
	productionDeployPassedQuery["type"] = config.JobZadigDeploy
	if len(projectName) != 0 {
		productionDeployPassedQuery["product_name"] = bson.M{"$in": projectName}
	}

	productionDeployPassedCount, err := c.CountDocuments(context.TODO(), productionDeployPassedQuery, &options.CountOptions{})
	if err != nil {
		return nil, err
	}

	resp.SuccessCount = productionDeployPassedCount

	timestampList := util.GetDailyStartTimestamps(startTime, endTime)

	dailyStat := make([]*DailyStat, 0)

	for i := 0; i < len(timestampList)-1; i++ {
		totalQuery := bson.M{}

		totalQuery["create_time"] = bson.M{"$gte": timestampList[i], "$lt": timestampList[i+1]}
		totalQuery["production"] = true
		// TODO: currently production deployment is only available for zadig deploy
		totalQuery["type"] = config.JobZadigDeploy
		if len(projectName) != 0 {
			totalQuery["product_name"] = projectName
		}

		currentDayDeployAmount, err := c.CountDocuments(context.TODO(), totalQuery, &options.CountOptions{})
		if err != nil {
			return nil, err
		}

		passedQuery := bson.M{}
		passedQuery["create_time"] = bson.M{"$gte": timestampList[i], "$lt": timestampList[i+1]}
		passedQuery["production"] = true
		// TODO: currently production deployment is only available for zadig deploy
		passedQuery["type"] = config.JobZadigDeploy
		passedQuery["status"] = config.StatusPassed
		if len(projectName) != 0 {
			passedQuery["product_name"] = bson.M{"$in": projectName}
		}

		currentDayDeployPassedAmount, err := c.CountDocuments(context.TODO(), passedQuery, &options.CountOptions{})
		if err != nil {
			return nil, err
		}

		failedQuery := bson.M{}
		failedQuery["create_time"] = bson.M{"$gte": timestampList[i], "$lt": timestampList[i+1]}
		failedQuery["production"] = true
		// TODO: currently production deployment is only available for zadig deploy
		failedQuery["type"] = config.JobZadigDeploy
		failedQuery["status"] = bson.M{"$in": []string{string(config.StatusFailed), string(config.StatusTimeout)}}
		if len(projectName) != 0 {
			failedQuery["product_name"] = bson.M{"$in": projectName}
		}

		currentDayDeployFailedAmount, err := c.CountDocuments(context.TODO(), passedQuery, &options.CountOptions{})
		if err != nil {
			return nil, err
		}

		dateInfo := time.Unix(timestampList[i], 0).Format("2006-01-02")
		dailyStat = append(dailyStat, &DailyStat{
			Date:         dateInfo,
			SuccessCount: currentDayDeployPassedAmount,
			FailCount:    currentDayDeployFailedAmount,
			Total:        currentDayDeployAmount,
		})
	}

	resp.DailyStat = dailyStat

	return resp, nil
}
