package mongodb

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type SonarIntegrationColl struct {
	*mongo.Collection

	coll string
}

func NewSonarIntegrationColl() *SonarIntegrationColl {
	name := models.SonarIntegration{}.TableName()
	return &SonarIntegrationColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *SonarIntegrationColl) GetCollectionName() string {
	return c.coll
}

func (c *SonarIntegrationColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *SonarIntegrationColl) Create(args *models.SonarIntegration) error {
	if args == nil {
		return errors.New("sonar integration is nil")
	}

	_, err := c.InsertOne(context.TODO(), args)

	return err
}

func (c *SonarIntegrationColl) List(pageNum, pageSize int64) ([]*models.SonarIntegration, int64, error) {
	query := bson.M{}
	resp := make([]*models.SonarIntegration, 0)
	ctx := context.Background()

	opt := options.Find()

	if pageNum != 0 && pageSize != 0 {
		opt = opt.
			SetSkip((pageNum - 1) * pageSize).
			SetLimit(pageSize)
	}

	cursor, err := c.Collection.Find(ctx, query, opt)
	if err != nil {
		return nil, 0, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, 0, err
	}
	count, err := c.Collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	return resp, count, nil
}

func (c *SonarIntegrationColl) GetByID(idstring string) (*models.SonarIntegration, error) {
	resp := new(models.SonarIntegration)
	id, err := primitive.ObjectIDFromHex(idstring)
	if err != nil {
		return nil, err
	}
	query := bson.M{"_id": id}

	err = c.FindOne(context.TODO(), query).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *SonarIntegrationColl) Update(idString string, obj *models.SonarIntegration) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return fmt.Errorf("invalid id")
	}
	filter := bson.M{"_id": id}
	update := bson.M{"$set": obj}

	_, err = c.UpdateOne(context.TODO(), filter, update)
	return err
}

func (c *SonarIntegrationColl) DeleteByID(idstring string) error {
	id, err := primitive.ObjectIDFromHex(idstring)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}
