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
	"reflect"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonoptions"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var once sync.Once
var client *mongo.Client
var isReplicaSet bool
var isReplicaSetChecked bool
var replicaSetMutex sync.RWMutex

func Database(name string) *mongo.Database {
	return Client().Database(name)
}

func SessionContext(ctx context.Context, session mongo.Session) context.Context {
	if !config.EnableTransaction() {
		return ctx
	}
	if session == nil {
		return ctx
	}
	return mongo.NewSessionContext(ctx, session)
}

func Session() mongo.Session {
	session, err := Client().StartSession()
	if err != nil {
		log.Panicf("Failed to start mongo session, err: %v", err)
		return nil
	}
	return session
}

func SessionWithTransaction(ctx context.Context) (mongo.Session, func(error), error) {
	session := Session()
	err := StartTransaction(session)
	if err != nil {
		return session, nil, errors.Wrap(err, "StartTransaction")
	}

	deferFunc := func(err error) {
		if err != nil {
			err = AbortTransaction(session)
			if err != nil {
				log.Errorf("Failed to abort transaction, err: %v", err)
			}
		}
		session.EndSession(ctx)
		return
	}

	return session, deferFunc, nil
}

func StartTransaction(session mongo.Session) error {
	if !config.EnableTransaction() {
		return nil
	}
	return session.StartTransaction()
}

func AbortTransaction(session mongo.Session) error {
	if !config.EnableTransaction() {
		return nil
	}
	return session.AbortTransaction(context.TODO())
}

func CommitTransaction(session mongo.Session) error {
	if !config.EnableTransaction() {
		return nil
	}
	return session.CommitTransaction(context.Background())
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

// IsReplicaSet checks if the MongoDB instance is running as a replica set.
// The result is cached after the first check for performance.
func IsReplicaSet(ctx context.Context) bool {
	replicaSetMutex.RLock()
	if isReplicaSetChecked {
		result := isReplicaSet
		replicaSetMutex.RUnlock()
		return result
	}
	replicaSetMutex.RUnlock()

	replicaSetMutex.Lock()
	defer replicaSetMutex.Unlock()

	// Double-check after acquiring write lock
	if isReplicaSetChecked {
		return isReplicaSet
	}

	// Run the "hello" command to check MongoDB topology
	// "hello" command is available since MongoDB 5.0, but also works on older versions
	var result bson.M
	err := Client().Database("admin").RunCommand(ctx, bson.D{{Key: "hello", Value: 1}}).Decode(&result)
	if err != nil {
		// Fallback to isMaster for older MongoDB versions
		err = Client().Database("admin").RunCommand(ctx, bson.D{{Key: "isMaster", Value: 1}}).Decode(&result)
		if err != nil {
			log.Warnf("Failed to detect MongoDB topology, assuming standalone: %v", err)
			isReplicaSetChecked = true
			isReplicaSet = false
			return false
		}
	}

	// Check if setName exists in the response - this indicates a replica set
	if setName, ok := result["setName"]; ok && setName != nil && setName != "" {
		isReplicaSet = true
	} else {
		isReplicaSet = false
	}
	isReplicaSetChecked = true

	log.Infof("MongoDB topology detected: isReplicaSet=%v", isReplicaSet)
	return isReplicaSet
}

// CreateIndexOptions returns the appropriate CreateIndexesOptions based on MongoDB topology.
// For replica sets, it sets commitQuorum to "majority" to prevent index creation from hanging
// when backup nodes fail. For standalone instances, it returns nil (no special options).
func CreateIndexOptions(ctx context.Context) *options.CreateIndexesOptions {
	if IsReplicaSet(ctx) {
		return options.CreateIndexes().SetCommitQuorumMajority()
	}
	return nil
}
