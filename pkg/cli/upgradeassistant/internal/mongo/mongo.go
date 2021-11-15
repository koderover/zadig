package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

func (c *CodehostColl) ChangeType(ID int, sourceType string) error {
	query := bson.M{"id": ID}

	preType := sourceType
	if sourceType == "1" {
		sourceType = "gitlab"
	} else if sourceType == "2" {
		sourceType = "github"
	} else if sourceType == "3" {
		sourceType = "gerrit"
	} else if sourceType == "4" {
		sourceType = "codehub"
	} else {
		return nil
	}

	change := bson.M{"$set": bson.M{
		"type": sourceType,
	}}
	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Errorf("repository update fail,err:%s", err)
		return err
	}
	log.Infof("success to change id:%d type:%s to type:%s", ID, preType, sourceType)
	return nil
}

func (c *CodehostColl) RollbackChangeType(ID int, sourceType string) error {
	query := bson.M{"id": ID}

	if sourceType == "gitlab" {
		sourceType = "1"
	} else if sourceType == "github" {
		sourceType = "2"
	} else if sourceType == "gerrit" {
		sourceType = "3"
	} else if sourceType == "codehub" {
		sourceType = "4"
	}

	change := bson.M{"$set": bson.M{
		"type": sourceType,
	}}
	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Errorf("repository update fail,err:%s", err)
		return err
	}
	log.Infof("success to rollback id:%d to type:%s", ID, sourceType)
	return nil
}

type CodeHost struct {
	ID            int    `bson:"id"               json:"id"`
	Type          string `bson:"type"             json:"type"`
	Address       string `bson:"address"          json:"address"`
	IsReady       string `bson:"is_ready"         json:"ready"`
	AccessToken   string `bson:"access_token"     json:"accessToken"`
	RefreshToken  string `bson:"refresh_token"    json:"refreshToken"`
	Namespace     string `bson:"namespace"        json:"namespace"`
	ApplicationId string `bson:"application_id"   json:"applicationId"`
	Region        string `bson:"region"           json:"region,omitempty"`
	Username      string `bson:"username"         json:"username,omitempty"`
	Password      string `bson:"password"         json:"password,omitempty"`
	ClientSecret  string `bson:"client_secret"    json:"clientSecret"`
	CreatedAt     int64  `bson:"created_at"       json:"created_at"`
	UpdatedAt     int64  `bson:"updated_at"       json:"updated_at"`
	DeletedAt     int64  `bson:"deleted_at"       json:"deleted_at"`
}

func (c *CodehostColl) ListCodeHosts() ([]*CodeHost, error) {
	codeHosts := make([]*CodeHost, 0)

	cursor, err := c.Collection.Find(context.TODO(), bson.M{})
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &codeHosts)
	if err != nil {
		return nil, err
	}
	return codeHosts, nil
}

type CodehostColl struct {
	*mongo.Collection

	coll string
}

type ListArgs struct {
	Owner   string
	Address string
	Source  string
}

func NewCodehostColl() *CodehostColl {
	name := models.CodeHost{}.TableName()
	coll := &CodehostColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}
