package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"

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

	_, err := c.InsertOne(context.TODO(), obj)

	return err
}
