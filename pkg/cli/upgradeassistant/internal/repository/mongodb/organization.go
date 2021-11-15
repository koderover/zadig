package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/pkg/config"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type OrganizationColl struct {
	*mongo.Collection

	coll string
}

func NewOrganizationColl() *OrganizationColl {
	name := models.Organization{}.TableName()
	return &OrganizationColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *OrganizationColl) GetCollectionName() string {
	return c.coll
}

func (c *OrganizationColl) EnsureIndex(_ context.Context) error {
	return nil
}

func (c *OrganizationColl) Get(id int) (*models.Organization, bool, error) {
	org := &models.Organization{}
	query := bson.M{"id": id}

	err := c.FindOne(context.TODO(), query).Decode(org)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, false, nil
		}
		return nil, false, err
	}
	return org, true, nil
}
