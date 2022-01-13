package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type PolicyDefineColl struct {
	*mongo.Collection

	coll string
}

func NewPolicyDefineColl() *PolicyDefineColl {
	name := models.PolicyDefine{}.TableName()
	return &PolicyDefineColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *PolicyDefineColl) GetCollectionName() string {
	return c.coll
}

func (c *PolicyDefineColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *PolicyDefineColl) Create(obj *models.PolicyDefine) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	obj.CreateTime = time.Now().Unix()
	_, err := c.InsertOne(context.TODO(), obj)

	return err
}

func (c *PolicyDefineColl) List(projectName string) ([]*models.PolicyDefine, error) {
	var res []*models.PolicyDefine

	ctx := context.Background()

	query := bson.M{"namespace": projectName}

	opts := options.Find()

	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *PolicyDefineColl) Delete(id string) error {

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}
