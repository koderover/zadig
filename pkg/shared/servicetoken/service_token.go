/*
Copyright 2026 The KodeRover Authors.

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

package servicetoken

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

const collectionName = "internal_service_token"

// InternalServiceToken is the persisted form of a per-service internal token. Name is the
// document ID so concurrent service replicas cannot create different tokens for the same service.
type InternalServiceToken struct {
	Name      string `bson:"_id"`
	Token     string `bson:"token"`
	TokenHash string `bson:"token_hash"`
}

var tokenCache sync.Map

// GetInternalToken returns the persisted opaque token for a service, creating it when needed.
// Service replicas with the same executable name share the same database record and token.
func GetInternalToken(name string) (string, error) {
	if name == "" {
		return "", errors.New("empty service name")
	}
	if token, ok := tokenCache.Load(name); ok {
		return token.(string), nil
	}

	coll := mongotool.Database(config.MongoDatabase()).Collection(collectionName)

	var existing InternalServiceToken
	err := coll.FindOne(context.TODO(), bson.M{"_id": name}).Decode(&existing)
	if err == nil {
		return cacheToken(existing)
	} else if err != mongo.ErrNoDocuments {
		return "", err
	}

	token, err := newToken()
	if err != nil {
		return "", err
	}
	_, err = coll.UpdateOne(
		context.TODO(),
		bson.M{"_id": name},
		bson.M{"$setOnInsert": bson.M{"token": token, "token_hash": crypto.Sha256([]byte(token))}},
		options.Update().SetUpsert(true),
	)
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		return "", err
	}

	// Read the winner back. This also handles another replica inserting the same service first.
	if err := coll.FindOne(context.TODO(), bson.M{"_id": name}).Decode(&existing); err != nil {
		return "", err
	}
	return cacheToken(existing)
}

// CurrentInternalToken gets the token for the current Zadig binary. Container binaries have
// stable service names such as aslan, user, cron and init.
func CurrentInternalToken() (string, error) {
	return GetInternalToken(filepath.Base(os.Args[0]))
}

// ValidateServiceToken authenticates an internal token by possession and returns the service
// name it belongs to. An empty, unknown or forged token returns an error.
func ValidateServiceToken(token string) (string, error) {
	if token == "" {
		return "", errors.New("empty internal service token")
	}
	coll := mongotool.Database(config.MongoDatabase()).Collection(collectionName)
	var doc InternalServiceToken
	err := coll.FindOne(context.TODO(), bson.M{"token_hash": crypto.Sha256([]byte(token))}).Decode(&doc)
	if err != nil {
		return "", fmt.Errorf("internal service token not found: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(doc.Token), []byte(token)) != 1 {
		return "", errors.New("internal service token mismatch")
	}

	return doc.Name, nil
}

func cacheToken(doc InternalServiceToken) (string, error) {
	if crypto.Sha256([]byte(doc.Token)) != doc.TokenHash {
		return "", errors.New("internal service token is corrupted")
	}
	tokenCache.Store(doc.Name, doc.Token)
	return doc.Token, nil
}

func newToken() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}
