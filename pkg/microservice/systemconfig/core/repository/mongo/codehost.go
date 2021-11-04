package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/globalsign/mgo/bson"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/repository/models"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type CodehostColl struct {
	*mongo.Collection

	coll string
}

func NewCodehostColl() *CodehostColl {
	name := models.CodeHost{}.TableName()
	coll := &CodehostColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *CodehostColl) GetCollectionName() string {
	return c.coll
}
func (c *CodehostColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *CodehostColl) AddCodeHost(iCodeHost *models.CodeHost) (*models.CodeHost, error) {

	_, err := c.Collection.InsertOne(context.TODO(), iCodeHost)
	if err != nil {
		log.Error("repository AddCodeHost err : %v", err)
		return nil, err
	}
	return iCodeHost, nil
}

func (c *CodehostColl) DeleteCodeHostByID(ID int) error {
	query := bson.M{"id": ID, "deleted_at": 0}
	change := bson.M{"$set": bson.M{
		"deleted_at": time.Now().Unix(),
	}}

	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Error("repository DeleteCodeHostByID err : %v", err)
		return err
	}
	return nil
}

func (c *CodehostColl) GetCodeHostByID(ID int) (*models.CodeHost, error) {

	codehost := new(models.CodeHost)
	query := bson.M{"id": ID, "deleted_at": 0}
	err := c.Collection.FindOne(context.TODO(), query).Decode(codehost)
	if err != nil {
		log.Error("repository GetCodeHostByID err : %v", err)
		return nil, err
	}

	return codehost, nil
}

func (c *CodehostColl) FindCodeHosts() ([]*models.CodeHost, error) {
	codeHosts := make([]*models.CodeHost, 0)
	query := bson.M{"deleted_at": 0}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &codeHosts)
	if err != nil {
		return nil, err
	}
	return codeHosts, nil
}
