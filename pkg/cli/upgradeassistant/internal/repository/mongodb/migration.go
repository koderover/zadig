package mongodb

import (
	"context"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MigrationColl struct {
	*mongo.Collection

	coll string
}

func NewMigrationColl() *MigrationColl {
	name := models.Migration{}.TableName()
	coll := &MigrationColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *MigrationColl) GetMigrationInfo() (*models.Migration, error) {
	query := bson.M{}

	var resp *models.Migration
	ctx := context.Background()
	opts := options.FindOne()
	err := c.Collection.FindOne(ctx, query, opts).Decode(&resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *MigrationColl) UpdateMigrationStatus(id primitive.ObjectID, kvs map[string]interface{}) error {
	query := bson.M{"_id": id}
	change := bson.M{}
	for key, val := range kvs {
		change[key] = val
	}
	changeQuery := bson.M{"$set": change}
	_, err := c.Collection.UpdateOne(context.TODO(), query, changeQuery)
	return err
}

func (c *MigrationColl) UpdateMigrationError(id primitive.ObjectID, error string) error {
	query := bson.M{"_id": id}
	change := bson.M{"$set": bson.M{"error": error}}
	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *MigrationColl) InitializeMigrationInfo() error {
	resp := &models.Migration{
		SonarMigration:                   false,
		UpdateWorkflow340JobSpec:         false,
		UpdateWorkflow340JobTemplateSpec: false,
	}
	_, err := c.InsertOne(context.TODO(), resp, options.InsertOne())

	return err
}
