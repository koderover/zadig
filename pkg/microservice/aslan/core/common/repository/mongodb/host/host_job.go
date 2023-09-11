package host

import (
	"context"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/host"
	"github.com/koderover/zadig/pkg/setting"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type HostJobColl struct {
	*mongo.Collection

	coll string
}

func NewHostJobColl() *HostJobColl {
	name := host.HostJob{}.TableName()
	return &HostJobColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *HostJobColl) GetCollectionName() string {
	return c.coll
}

func (c *HostJobColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *HostJobColl) Create(obj *host.HostJob) error {
	if obj == nil {
		return nil
	}

	_, err := c.InsertOne(context.Background(), obj)
	return err
}

func (c *HostJobColl) Update(idString string, obj *host.HostJob) error {
	if obj == nil {
		return nil
	}

	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}

	query := bson.M{"_id": id}
	change := bson.M{"$set": obj}

	_, err = c.UpdateOne(context.Background(), query, change)
	return err
}

func (c *HostJobColl) FindByID(idString string) (*host.HostJob, error) {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return nil, err
	}

	query := bson.M{"_id": id}

	res := &host.HostJob{}
	err = c.FindOne(context.Background(), query).Decode(res)
	return res, err
}

func (c *HostJobColl) FindOldestByTags(tags []string) (*host.HostJob, error) {
	query := bson.M{
		"tags":   bson.M{"$in": tags},
		"status": setting.HostJobStatusCreated,
	}

	opts := options.FindOne()
	opts.SetSort(bson.M{"created_time": 1})

	res := &host.HostJob{}
	err := c.FindOne(context.Background(), query, opts).Decode(res)
	return res, err
}

type HostJobOpts struct {
	Names []string
}

func (c *HostJobColl) ListByOpts(opts *HostJobOpts) ([]*host.HostJob, error) {
	query := bson.M{}
	if len(opts.Names) > 0 {
		query["name"] = bson.M{"$in": opts.Names}
	}

	res := make([]*host.HostJob, 0)
	cursor, err := c.Collection.Find(context.Background(), query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.Background(), &res)
	return res, err
}
