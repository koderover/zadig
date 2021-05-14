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
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RegistryNamespace struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"               json:"id,omitempty"`
	OrgID            int                `bson:"org_id"                      json:"org_id"`
	RegAddr          string             `bson:"reg_addr"                    json:"reg_addr"`
	RegType          string             `bson:"reg_type"                    json:"reg_type"`
	RegProvider      string             `bson:"reg_provider"                json:"reg_provider"`
	IsDefault        bool               `bson:"is_default"                  json:"is_default"`
	Namespace        string             `bson:"namespace"                   json:"namespace"`
	AccessKey        string             `bson:"access_key"                  json:"access_key"`
	SecretyKey       string             `bson:"secret_key"                  json:"secret_key"`
	TencentSecretID  string             `bson:"tencent_secret_id"           json:"tencent_secret_id"`
	TencentSecretKey string             `bson:"tencent_secret_key"          json:"tencent_secret_key"`
	UpdateTime       int64              `bson:"update_time"                 json:"update_time"`
	UpdateBy         string             `bson:"update_by"                   json:"update_by"`
}

func (ns *RegistryNamespace) Validate() error {
	if ns.OrgID == 0 {
		return errors.New("empty org_id")
	}

	if ns.RegAddr == "" {
		return errors.New("empty reg_addr")
	}

	if ns.RegProvider == "" {
		return errors.New("empty reg_provider")
	}

	if ns.Namespace == "" {
		return errors.New("empty namespace")
	}

	return nil
}

func (RegistryNamespace) TableName() string {
	return "registry_namespace"
}

//const CandidateRegType = "candidate"
//
//const DistRegType = "dist"
