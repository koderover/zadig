package mongodb

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type PrivateKeyProjectRelationColl struct {
	*mongo.Collection

	coll string
}

func NewPrivateKeyProjectRelationColl() *PrivateKeyProjectRelationColl {
	name := models.PrivateKeyProjectRelation{}.TableName()
	return &PrivateKeyProjectRelationColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *PrivateKeyProjectRelationColl) GetCollectionName() string {
	return c.coll
}

type PrivateKeyProjectRelationOption struct {
	Name        string `json:"name"`
	ProjectName string `json:"project_name"`
}

func (c *PrivateKeyProjectRelationColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "name", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)
	return err
}

func (c *PrivateKeyProjectRelationColl) List(args *PrivateKeyProjectRelationOption) ([]*models.PrivateKeyProjectRelation, error) {
	query := bson.M{}
	if args.Name != "" {
		query["name"] = args.Name
	}

	if args.ProjectName != "" {
		query["project_name"] = args.ProjectName
	}

	var resp []*models.PrivateKeyProjectRelation
	ctx := context.Background()

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *PrivateKeyProjectRelationColl) Create(args *models.PrivateKeyProjectRelation) error {
	if args == nil {
		return errors.New("nil privateKeyProjectRelation info")
	}

	_, err := c.InsertOne(context.TODO(), args)

	return err
}

func (c *PrivateKeyProjectRelationColl) Delete(args *PrivateKeyProjectRelationOption) error {
	query := bson.M{}
	if args.Name != "" {
		query["name"] = args.Name
	}

	if args.ProjectName != "" {
		query["project_name"] = args.ProjectName
	}

	_, err := c.DeleteOne(context.TODO(), query)
	return err
}
