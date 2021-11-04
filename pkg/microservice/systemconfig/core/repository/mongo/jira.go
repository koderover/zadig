package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/globalsign/mgo/bson"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/repository/models"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type JiraColl struct {
	*mongo.Collection

	coll string
}

func NewJiraColl() *JiraColl {
	name := models.Jira{}.TableName()
	coll := &JiraColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *JiraColl) GetCollectionName() string {
	return c.coll
}
func (c *JiraColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *JiraColl) AddJira(iJira *models.Jira) (*models.Jira, error) {
	_, err := c.Collection.InsertOne(context.TODO(), iJira)
	if err != nil {
		log.Error("repository AddJira err : %v", err)
		return nil, err
	}
	return iJira, nil
}

func (c *JiraColl) UpdateJira(iJira *models.Jira) (*models.Jira, error) {

	query := bson.M{"deleted_at": 0}
	change := bson.M{"$set": bson.M{
		"host":         iJira.Host,
		"user":         iJira.User,
		"access_token": iJira.AccessToken,
		"updated_at":   time.Now().Unix(),
	}}

	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Error("repository UpdateJira err : %v", err)
		return nil, err
	}
	return iJira, nil
}

func (c *JiraColl) DeleteJira() error {

	query := bson.M{"deleted_at": 0}
	change := bson.M{"$set": bson.M{
		"deleted_at": time.Now().Unix(),
	}}

	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Error("repository DeleteJira err : %v", err)
		return err
	}
	return nil
}

func (c *JiraColl) GetJira() (*models.Jira, error) {
	jira := &models.Jira{}
	query := bson.M{"deleted_at": 0}

	err := c.Collection.FindOne(context.TODO(), query).Decode(jira)
	if err != nil {
		log.Error("repository GetJira err : %v", err)
		return nil, err
	}
	return jira, nil
}
