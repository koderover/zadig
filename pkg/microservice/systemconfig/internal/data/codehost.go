package data

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/internal/biz"
)

var _ biz.CodeHostRepo = (*codeHostRepo)(nil)

type codeHostRepo struct {
	data         *Data
	codeHostColl *mongo.Collection
}

type CodeHost struct {
	ID            int    `bson:"id"`
	Type          string `bson:"type"`
	Address       string `bson:"address"`
	IsReady       string `bson:"is_ready"`
	AccessToken   string `bson:"access_token"`
	RefreshToken  string `bson:"refresh_token"`
	Namespace     string `bson:"namespace"`
	ApplicationId string `bson:"application_id"`
	Region        string `bson:"region"`
	Username      string `bson:"username"`
	Password      string `bson:"password"`
	ClientSecret  string `bson:"client_secret"`
	CreatedAt     int64  `bson:"created_at"`
	UpdatedAt     int64  `bson:"updated_at"`
	DeletedAt     int64  `bson:"deleted_at"`
}

func NewCodeHostRepo(data *Data) biz.CodeHostRepo {
	return &codeHostRepo{
		data:         data,
		codeHostColl: data.db.Collection("code_host"),
	}
}

func (r *codeHostRepo) GetCodeHost(ctx context.Context, id int) (*biz.CodeHost, error) {
	result := &CodeHost{}
	if err := r.codeHostColl.FindOne(ctx, bson.M{"id": id, "deleted_at": 0}).Decode(&result); err != nil {
		if err == mongo.ErrNoDocuments {
			return &biz.CodeHost{ID: result.ID}, nil
		}
		return nil, err
	}
	return &biz.CodeHost{
		ID:            result.ID,
		Type:          result.Type,
		Address:       result.Address,
		IsReady:       result.IsReady,
		AccessToken:   result.AccessToken,
		RefreshToken:  result.RefreshToken,
		Namespace:     result.Namespace,
		ApplicationId: result.ApplicationId,
		Region:        result.Region,
		Username:      result.Username,
		Password:      result.Password,
		ClientSecret:  result.ClientSecret,
		CreatedAt:     result.CreatedAt,
		UpdatedAt:     result.UpdatedAt,
		DeletedAt:     result.DeletedAt,
	}, nil
}
