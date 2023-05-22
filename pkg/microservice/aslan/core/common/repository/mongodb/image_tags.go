package mongodb

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type ImageTagsColl struct {
	*mongo.Collection

	coll string
}

func NewImageTagsCollColl() *ImageTagsColl {
	name := models.ImageTags{}.TableName()
	return &ImageTagsColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ImageTagsColl) GetCollectionName() string {
	return c.coll
}

type ImageTagsFindOption struct {
	RegistryID  string
	RegProvider string
	ImageName   string
	Namespace   string
	TagName     string
}

func (c *ImageTagsColl) Find(opt *ImageTagsFindOption) (*models.ImageTags, error) {
	tags := &models.ImageTags{}
	query := bson.M{}
	if opt.ImageName != "" {
		query["image_name"] = opt.ImageName
	}
	if opt.TagName != "" {
		query["image_tags.tag_name"] = opt.TagName
	}
	if opt.Namespace != "" {
		query["namespace"] = opt.Namespace
	}
	if opt.RegistryID != "" {
		query["registry_id"] = opt.RegistryID
	}
	if opt.RegProvider != "" {
		query["reg_provider"] = opt.RegProvider
	}

	err := c.FindOne(context.Background(), query).Decode(tags)
	return tags, err
}

func (c *ImageTagsColl) Insert(args *models.ImageTags) error {
	if args == nil {
		return errors.New("nil image_tag args")
	}

	result, err := c.InsertOne(context.TODO(), args)
	if err != nil || result == nil {
		return err
	}

	return nil
}

func (c *ImageTagsColl) UpdateOrInsert(args *models.ImageTags) error {
	if args == nil {
		return errors.New("nil image_tag args")
	}

	filter := bson.D{
		{"image_name", args.ImageName},
		{"namespace", args.Namespace},
		{"reg_provider", args.RegProvider},
		{"registry_id", args.RegistryID},
	}

	update := bson.D{
		{"$push", bson.D{
			{"image_tags", bson.D{
				{"$each", args.ImageTags},
			}},
		}},
	}

	result, err := c.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		_, err = c.InsertOne(context.TODO(), args)
		if err != nil {
			return err
		}
	}

	return nil
}
