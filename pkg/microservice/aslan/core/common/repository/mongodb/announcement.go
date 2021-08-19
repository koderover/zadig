package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type AnnouncementColl struct {
	*mongo.Collection

	coll string
}

func NewAnnouncementColl() *AnnouncementColl {
	name := models.Announcement{}.TableName()
	return &AnnouncementColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *AnnouncementColl) GetCollectionName() string {
	return c.coll
}

func (c *AnnouncementColl) EnsureIndex(_ context.Context) error {
	return nil
}

func (c *AnnouncementColl) Create(args *models.Announcement) error {
	args.CreateTime = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)

	return err
}

func (c *AnnouncementColl) Update(id string, args *models.Announcement) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = c.UpdateByID(context.TODO(), oid, bson.M{"$set": args})

	return err
}

func (c *AnnouncementColl) List(receiver string) ([]*models.Announcement, error) {
	var res []*models.Announcement

	query := bson.M{"receiver": receiver}
	opts := options.Find().SetSort(bson.D{{"create_time", -1}}).SetLimit(100)
	cursor, err := c.Collection.Find(context.TODO(), query, opts)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *AnnouncementColl) ListValidAnnouncements(receiver string) ([]*models.Announcement, error) {
	var res []*models.Announcement

	query := bson.M{"receiver": receiver}
	now := time.Now().Unix()
	query["content.start_time"] = bson.M{"$lt": now}
	query["content.end_time"] = bson.M{"$gt": now}

	opts := options.Find().SetSort(bson.D{{"create_time", -1}}).SetLimit(100)
	cursor, err := c.Collection.Find(context.TODO(), query, opts)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}
