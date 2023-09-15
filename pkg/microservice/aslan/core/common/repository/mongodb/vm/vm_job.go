package vm

import (
	"context"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/vm"
	"github.com/koderover/zadig/pkg/setting"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type VMJobColl struct {
	*mongo.Collection

	coll string
}

func NewVMJobColl() *VMJobColl {
	name := vm.VMJob{}.TableName()
	return &VMJobColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *VMJobColl) GetCollectionName() string {
	return c.coll
}

func (c *VMJobColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *VMJobColl) Create(obj *vm.VMJob) error {
	if obj == nil {
		return nil
	}

	_, err := c.InsertOne(context.Background(), obj)
	return err
}

func (c *VMJobColl) Update(idString string, obj *vm.VMJob) error {
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

func (c *VMJobColl) FindByID(idString string) (*vm.VMJob, error) {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return nil, err
	}

	query := bson.M{"_id": id}

	res := &vm.VMJob{}
	err = c.FindOne(context.Background(), query).Decode(res)
	return res, err
}

func (c *VMJobColl) FindOldestByTags(tags []string) (*vm.VMJob, error) {
	query := bson.M{
		"tags":   bson.M{"$in": tags},
		"status": setting.VMJobStatusCreated,
	}

	opts := options.FindOne()
	opts.SetSort(bson.M{"created_time": 1})

	res := &vm.VMJob{}
	err := c.FindOne(context.Background(), query, opts).Decode(res)
	return res, err
}

type VMJobOpts struct {
	Names []string
}

func (c *VMJobColl) ListByOpts(opts *VMJobOpts) ([]*vm.VMJob, error) {
	query := bson.M{}
	if len(opts.Names) > 0 {
		query["name"] = bson.M{"$in": opts.Names}
	}

	res := make([]*vm.VMJob, 0)
	cursor, err := c.Collection.Find(context.Background(), query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.Background(), &res)
	return res, err
}
