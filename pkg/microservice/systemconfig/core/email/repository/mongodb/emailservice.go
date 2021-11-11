package mongodb

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/repository/models"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type EmailServiceColl struct {
	*mongo.Collection

	coll string
}

func NewEmailServiceColl() *EmailServiceColl {
	name := models.EmailService{}.TableName()
	coll := &EmailServiceColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *EmailServiceColl) GetCollectionName() string {
	return c.coll
}
func (c *EmailServiceColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *EmailServiceColl) AddEmailService(iEmailService *models.EmailService) (*models.EmailService, error) {
	// TODO: tmp solution to avoid bug
	var res []*models.EmailService
	query := bson.M{"deleted_at": 0}
	cur, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cur.All(context.TODO(), &res)
	if err != nil {
		return nil, err
	}
	if len(res) >= 1 {
		return nil, errors.New("cant add more than one emailservice")
	}

	_, err = c.Collection.InsertOne(context.TODO(), iEmailService)
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
