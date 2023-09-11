package host

import (
	"context"
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/host"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type ZadigHostColl struct {
	*mongo.Collection

	coll string
}

func NewZadigHostColl() *ZadigHostColl {
	name := host.ZadigHost{}.TableName()
	return &ZadigHostColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ZadigHostColl) GetCollectionName() string {
	return c.coll
}

func (c *ZadigHostColl) EnsureIndex(ctx context.Context) error {
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

func (c *ZadigHostColl) Create(obj *host.ZadigHost) error {
	if obj == nil {
		return nil
	}

	_, err := c.InsertOne(context.Background(), obj)
	return err
}

func (c *ZadigHostColl) Update(idString string, obj *host.ZadigHost) error {
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

func (c *ZadigHostColl) Delete(idString string) error {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *ZadigHostColl) FindByToken(token string) (*host.ZadigHost, error) {
	query := bson.M{"token": token}
	res := &host.ZadigHost{}
	err := c.FindOne(context.Background(), query).Decode(res)
	return res, err
}

func (c *ZadigHostColl) FindByName(name string) (*host.ZadigHost, error) {
	query := bson.M{
		"name":       name,
		"is_deleted": false,
	}
	res := &host.ZadigHost{}
	err := c.FindOne(context.Background(), query).Decode(res)
	return res, err
}

func (c *ZadigHostColl) FindByID(idString string) (*host.ZadigHost, error) {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return nil, err
	}
	query := bson.M{
		"_id":        id,
		"is_deleted": false,
	}
	res := &host.ZadigHost{}
	err = c.FindOne(context.Background(), query).Decode(res)
	return res, err
}

type ZadigHostListOptions struct {
	Name     []string
	PageNum  int64
	PageSize int64
}

func (c *ZadigHostColl) ListByOptions(opt ZadigHostListOptions) ([]*host.ZadigHost, int64, error) {
	query := bson.M{}
	if len(opt.Name) > 0 {
		query["name"] = bson.M{"$in": opt.Name}
	}
	query["is_deleted"] = false

	total, err := c.CountDocuments(context.Background(), query)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find()
	if opt.PageNum > 0 && opt.PageSize > 0 {
		opts.SetSkip((opt.PageNum - 1) * opt.PageSize)
		opts.SetLimit(opt.PageSize)
	}

	res := make([]*host.ZadigHost, 0)
	cursor, err := c.Collection.Find(context.Background(), query, opts)
	if err != nil {
		return nil, 0, err
	}

	for cursor.Next(context.Background()) {
		agent := new(host.ZadigHost)
		if err := cursor.Decode(agent); err != nil {
			return nil, 0, err
		}
		res = append(res, agent)
	}

	return res, total, nil
}
