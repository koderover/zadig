/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mongo

import (
	"context"
	"log"
	"reflect"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonoptions"
	"go.mongodb.org/mongo-driver/bson/bsontype"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var once sync.Once
var client *mongo.Client

func Database(name string) *mongo.Database {
	return Client().Database(name)
}

func Client() *mongo.Client {
	if client == nil {
		panic("mongoDB connection is not initialized yet")
	}
	return client
}

// Init is a singleton, it will be initialized only once.
// In case the uri provides only a single host in the mongodb cluster, the system will
// attempt to connect without discovering other hosts in the cluster.
func Init(ctx context.Context, uri string) {
	once.Do(func() {
		nilSliceCodec := bsoncodec.NewSliceCodec(bsonoptions.SliceCodec().SetEncodeNilAsEmpty(true))
		tM := reflect.TypeOf(bson.M{})
		reg := bson.NewRegistryBuilder().RegisterTypeMapEntry(bsontype.EmbeddedDocument, tM).RegisterDefaultEncoder(reflect.Slice, nilSliceCodec).Build()

		connInfo, err := extractURL(uri)
		if err != nil {
			log.Fatalf("Failed to initialize mongo db connection, err: %v", err)
		}
		opt := options.Client().ApplyURI(uri).SetRegistry(reg)
		// By default the client will discover the mongodb cluster topology (if exists) and try to
		// connect to ALL hosts in the cluster.
		// If NONE of the host is discoverable by its host name (private network host name),
		// and only a single host ip is provided, the auto-discovery function will cause a panic due
		// to non of the host can be connected by the discovered host name.
		// Thus, when there is only 1 addr in the provided uri, the system will try to connect with
		// the given connection string ONLY.
		// ref: https://pkg.go.dev/go.mongodb.org/mongo-driver/mongo/options#ClientOptions.SetDirect
		if len(connInfo.addrs) == 1 {
			opt.SetDirect(true)
		}
		client = connect(ctx, opt)
	})
}

// InitWithOption is a singleton, it will be initialized only once.
func InitWithOption(ctx context.Context, opt *options.ClientOptions) {
	once.Do(func() {
		tM := reflect.TypeOf(bson.M{})
		reg := bson.NewRegistryBuilder().RegisterTypeMapEntry(bsontype.EmbeddedDocument, tM).Build()
		opt.SetRegistry(reg)
		client = connect(ctx, opt)
	})
}

func Close(ctx context.Context) error {
	return client.Disconnect(ctx)
}

func Ping(ctx context.Context) error {
	return client.Ping(ctx, readpref.Primary())
}

func connect(ctx context.Context, opt *options.ClientOptions) *mongo.Client {
	c, err := mongo.Connect(ctx, opt)
	if err != nil {
		panic(err)
	}

	return c
}
