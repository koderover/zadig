package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/mongo"
)

type LabelColl struct {
	*mongo.Collection
	coll string
}

func NewLabelColl() *LabelColl {
	name := models.Label{}.TableName()
	return &LabelColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *LabelColl) GetCollectionName() string {
	return c.coll
}

func (c *LabelColl) EnsureIndex(ctx context.Context) error {
	// currently no query is required for the label defining collection
	return nil
}

func (c *LabelColl) Create(args *models.Label) error {
	if args == nil {
		return fmt.Errorf("given label is nil")
	}

	args.CreatedAt = time.Now().Unix()
	_, err := c.InsertOne(context.TODO(), args)
	return err
}
