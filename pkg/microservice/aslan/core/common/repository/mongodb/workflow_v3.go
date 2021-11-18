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

type WorkflowV3Coll struct {
	*mongo.Collection

	coll string
}

type ListWorkflowV3Option struct {
	ProjectName string
}

func NewWorkflowV3Coll() *WorkflowV3Coll {
	name := models.WorkflowV3{}.TableName()
	return &WorkflowV3Coll{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *WorkflowV3Coll) GetCollectionName() string {
	return c.coll
}

func (c *WorkflowV3Coll) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"project_name": 1},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *WorkflowV3Coll) Create(obj *models.WorkflowV3) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	_, err := c.InsertOne(context.TODO(), obj)
	return err
}

func (c *WorkflowV3Coll) List(opt *ListWorkflowV3Option, pageNum, pageSize int64) ([]*models.WorkflowV3, int64, error) {
	resp := make([]*models.WorkflowV3, 0)
	query := bson.M{}
	if opt.ProjectName != "" {
		query["project_name"] = opt.ProjectName
	}
	count, err := c.CountDocuments(context.TODO(), query)
	if err != nil {
		return nil, 0, err
	}
	findOption := options.Find().
		SetSkip((pageNum - 1) * pageSize).
		SetLimit(pageSize)

	cursor, err := c.Collection.Find(context.TODO(), query, findOption)
	if err != nil {
		return nil, 0, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, 0, err
	}
	return resp, count, nil
}

func (c *WorkflowV3Coll) GetByID(idstring string) (*models.WorkflowV3, error) {
	resp := new(models.WorkflowV3)
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

func (c *WorkflowV3Coll) Update(idString string, obj *models.WorkflowV3) error {
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

func (c *WorkflowV3Coll) DeleteByID(idString string) error {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}
