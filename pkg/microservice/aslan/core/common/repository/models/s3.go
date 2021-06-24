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

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type S3Storage struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"  json:"id"`
	Ak          string             `bson:"ak"             json:"ak"`
	Sk          string             `bson:"-"              json:"sk"`
	Endpoint    string             `bson:"endpoint"       json:"endpoint"`
	Bucket      string             `bson:"bucket"         json:"bucket"`
	Subfolder   string             `bson:"subfolder"      json:"subfolder"`
	Insecure    bool               `bson:"insecure"       json:"insecure"`
	IsDefault   bool               `bson:"is_default"     json:"is_default"`
	EncryptedSk string             `bson:"encryptedSk"    json:"-"`
	UpdatedBy   string             `bson:"updated_by"     json:"updated_by"`
	UpdateTime  int64              `bson:"update_time"    json:"update_time"`
}

func (s S3Storage) TableName() string {
	return "s3storage"
}
