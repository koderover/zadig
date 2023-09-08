package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type ZadigAgentColl struct {
	*mongo.Collection

	coll string
}

func NewZadigAgentColl() *ZadigAgentColl {
	name := models.ZadigAgent{}.TableName()
	return &ZadigAgentColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ZadigAgentColl) GetCollectionName() string {
	return c.coll
}

func (c *ZadigAgentColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
			bson.E{Key: "token", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *ZadigAgentColl) Create(obj *models.ZadigAgent) error {
	if obj == nil {
		return nil
	}

	_, err := c.InsertOne(context.Background(), obj)
	return err
}

func (c *ZadigAgentColl) Update(idString string, obj *models.ZadigAgent) error {
	if obj == nil {
		return nil
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

func (c *ZadigAgentColl) Delete(idString string) error {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *ZadigAgentColl) FindByToken(token string) (*models.ZadigAgent, error) {
	query := bson.M{"token": token}
	res := &models.ZadigAgent{}
	err := c.FindOne(context.Background(), query).Decode(res)
	return res, err
}
