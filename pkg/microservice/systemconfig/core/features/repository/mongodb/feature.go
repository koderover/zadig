package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/features/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type FeatureColl struct {
	*mongo.Collection

	coll string
}

func NewFeatureColl() *FeatureColl {
	name := models.Feature{}.TableName()
	coll := &FeatureColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *FeatureColl) GetCollectionName() string {
	return c.coll
}

func (c *FeatureColl) UpdateOrCreateFeature(ctx context.Context, obj *models.Feature) error {
	q := bson.M{"name": obj.Name}
	opts := options.Replace().SetUpsert(true)
	_, err := c.ReplaceOne(ctx, q, obj, opts)
	return err
}

func (c *FeatureColl) ListFeatures() ([]*models.Feature, error) {
	var fs []*models.Feature
	cursor, err := c.Find(context.TODO(), bson.M{})
	if err != nil {
		return nil, err
	}
	if err := cursor.All(context.TODO(), &fs); err != nil {
		return nil, err
	}
	return fs, nil
}
