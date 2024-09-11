package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type LabelColl struct {
	*mongo.Collection
	coll string
}

func NewLabelColl() *LabelColl {
	name := models.Label{}.TableName()
	return &LabelColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *LabelColl) GetCollectionName() string {
	return c.coll
}

func (c *LabelColl) EnsureIndex(ctx context.Context) error {
	// currently no query is required for the label defining collection
	return nil
}

func (c *LabelColl) Create(args *models.Label) error {
	if args == nil {
		return fmt.Errorf("given label is nil")
	}

	args.CreatedAt = time.Now().Unix()
	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *LabelColl) List() ([]*models.Label, error) {
	var clusters []*models.Label

	query := bson.M{}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &clusters)
	if err != nil {
		return nil, err
	}

	return clusters, err
}

func (c *LabelColl) Update(id string, args *models.Label) error {
	labelID, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		return err
	}
	_, err = c.UpdateOne(context.TODO(),
		bson.M{"_id": labelID}, bson.M{"$set": bson.M{
			"key":         args.Key,
			"description": args.Description,
			"updated_at":  args.UpdatedAt,
		}},
	)

	return err
}

func (c *LabelColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}
