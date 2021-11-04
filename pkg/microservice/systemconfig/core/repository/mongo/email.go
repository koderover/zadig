package mongo

import (
	"context"
	"time"

	"github.com/globalsign/mgo/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/repository/models"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type EmailHostColl struct {
	*mongo.Collection

	coll string
}

type EmailServiceColl struct {
	*mongo.Collection

	coll string
}

func NewEmailServiceColl() *EmailServiceColl {
	name := models.EmailHost{}.TableName()
	coll := &EmailServiceColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func NewEmailHostColl() *EmailHostColl {
	name := models.EmailHost{}.TableName()
	coll := &EmailHostColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *EmailServiceColl) GetCollectionName() string {
	return c.coll
}
func (c *EmailServiceColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *EmailHostColl) GetCollectionName() string {
	return c.coll
}
func (c *EmailHostColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *EmailHostColl) Find() (*models.EmailHost, error) {
	emailHost := new(models.EmailHost)
	query := bson.M{"deleted_at": 0}

	ctx := context.Background()

	err := c.Collection.FindOne(ctx, query).Decode(emailHost)
	if err != nil {
		return nil, nil
	}
	return emailHost, nil
}

func (c *EmailHostColl) Update(emailHost *models.EmailHost) (*models.EmailHost, error) {
	query := bson.M{"deleted_at": 0}
	change := bson.M{"$set": bson.M{
		"name":       emailHost.Name,
		"port":       emailHost.Port,
		"username":   emailHost.Username,
		"password":   emailHost.Password,
		"is_tls":     emailHost.IsTLS,
		"updated_at": time.Now().Unix(),
	}}

	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Error("repository Update EmailHostColl err : %v", err)
		return nil, err
	}
	return emailHost, nil
}

func (c *EmailHostColl) Delete() error {
	query := bson.M{"deleted_at": 0}
	change := bson.M{"$set": bson.M{
		"deleted_at": time.Now().Unix(),
	}}

	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Error("repository Delete EmailHostColl err : %v", err)
		return err
	}
	return nil
}

func (c *EmailHostColl) Add(emailHost *models.EmailHost) (*models.EmailHost, error) {

	_, err := c.Collection.InsertOne(context.TODO(), emailHost)
	if err != nil {
		log.Error("repository AddEmailHost err : %v", err)
		return nil, err
	}
	return emailHost, nil
}

func (c *EmailServiceColl) AddEmailService(iEmailService *models.EmailService) (*models.EmailService, error) {

	_, err := c.Collection.InsertOne(context.TODO(), iEmailService)
	if err != nil {
		log.Error("repository AddEmailService err : %v", err)
		return nil, err
	}
	return iEmailService, nil
}

func (c *EmailServiceColl) GetEmailService() (*models.EmailService, error) {
	query := bson.M{"deleted_at": 0}
	iEmailService := &models.EmailService{}
	err := c.Collection.FindOne(context.TODO(), query).Decode(iEmailService)
	if err != nil {
		return nil, nil
	}
	return iEmailService, nil
}

func (c *EmailServiceColl) UpdateEmailService(iEmailService *models.EmailService) (*models.EmailService, error) {
	query := bson.M{"deleted_at": 0}
	change := bson.M{"$set": bson.M{
		"name":         iEmailService.Name,
		"address":      iEmailService.Address,
		"display_name": iEmailService.DisplayName,
		"theme":        iEmailService.Theme,
		"updated_at":   time.Now().Unix(),
	}}

	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Error("repository UpdateEmailService err : %v", err)
		return nil, err
	}
	return iEmailService, nil
}

func (c *EmailServiceColl) DeleteEmailService() error {
	query := bson.M{"deleted_at": 0}
	change := bson.M{"$set": bson.M{
		"deleted_at": time.Now().Unix(),
	}}
	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Error("repository DeleteEmailServiceByOrgID err : %v", err)
		return err
	}
	return nil
}
