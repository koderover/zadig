package data

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Data struct {
	db *mongo.Database
}

// NewMongo
func NewMongo() *mongo.Database {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27019"))
	if err != nil {
		panic(err)
	}
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		panic(err)
	}
	return client.Database("testing")
}

// NewData
func NewData(database *mongo.Database) (*Data, func(), error) {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	d := &Data{
		db: database,
	}
	return d, func() {
		if err := d.db.Client().Disconnect(ctx); err != nil {
			log.Fatal("disconnect error")
		}
	}, nil
}
