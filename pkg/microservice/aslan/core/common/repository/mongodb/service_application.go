/*
Copyright 2025 The KodeRover Authors.

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

package mongodb

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
)

// UpdateServiceApplicationLinks updates the application links for a testing service
func (c *ServiceColl) UpdateServiceApplicationLinks(serviceName, productName string, applicationID *primitive.ObjectID) error {
	if serviceName == "" {
		return fmt.Errorf("serviceName is empty")
	}
	if productName == "" {
		return fmt.Errorf("productName is empty")
	}

	query := bson.M{"service_name": serviceName, "product_name": productName}
	changeMap := bson.M{}

	if applicationID != nil {
		changeMap["application_id"] = applicationID
	} else {
		changeMap["application_id"] = nil
	}

	change := bson.M{"$set": changeMap}
	_, err := c.UpdateMany(context.TODO(), query, change)
	return err
}

// ClearServiceApplicationLinks removes application links from a testing service
func (c *ServiceColl) ClearServiceApplicationLinks(serviceName, productName string) error {
	if serviceName == "" {
		return fmt.Errorf("serviceName is empty")
	}
	if productName == "" {
		return fmt.Errorf("productName is empty")
	}

	query := bson.M{"service_name": serviceName, "product_name": productName}
	changeMap := bson.M{
		"application_id": nil,
	}

	change := bson.M{"$set": changeMap}
	_, err := c.UpdateMany(context.TODO(), query, change)
	return err
}

// FindServiceByApplicationLink finds testing services linked to a specific application
func (c *ServiceColl) FindServiceByApplicationLink(applicationID primitive.ObjectID) ([]*models.Service, error) {
	query := bson.M{
		"application_id": applicationID,
		"status":         bson.M{"$ne": setting.ProductStatusDeleting},
	}

	var services []*models.Service
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.TODO(), &services)
	if err != nil {
		return nil, err
	}

	return services, nil
}

// CheckServiceApplicationConflict checks if a testing service is already linked to another application
func (c *ServiceColl) CheckServiceApplicationConflict(serviceName, productName string, applicationID primitive.ObjectID) (*models.Service, error) {
	query := bson.M{
		"service_name":   serviceName,
		"product_name":   productName,
		"application_id": bson.M{"$exists": true, "$nin": bson.A{nil, applicationID}},
		"status":         bson.M{"$ne": setting.ProductStatusDeleting},
	}

	var service models.Service
	err := c.FindOne(context.TODO(), query).Decode(&service)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // no conflict
		}
		return nil, err
	}

	return &service, nil
}

// Production Service Application Linking Methods

// UpdateProductionServiceApplicationLinks updates the application links for a production service
func (c *ProductionServiceColl) UpdateProductionServiceApplicationLinks(serviceName, productName string, applicationID *primitive.ObjectID) error {
	if serviceName == "" {
		return fmt.Errorf("serviceName is empty")
	}
	if productName == "" {
		return fmt.Errorf("productName is empty")
	}

	query := bson.M{"service_name": serviceName, "product_name": productName}
	changeMap := bson.M{}

	if applicationID != nil {
		changeMap["application_id"] = applicationID
	} else {
		changeMap["application_id"] = nil
	}

	change := bson.M{"$set": changeMap}
	_, err := c.UpdateMany(context.TODO(), query, change)
	return err
}

// ClearProductionServiceApplicationLinks removes application links from a production service
func (c *ProductionServiceColl) ClearProductionServiceApplicationLinks(serviceName, productName string) error {
	if serviceName == "" {
		return fmt.Errorf("serviceName is empty")
	}
	if productName == "" {
		return fmt.Errorf("productName is empty")
	}

	query := bson.M{"service_name": serviceName, "product_name": productName}
	changeMap := bson.M{
		"application_id": nil,
	}

	change := bson.M{"$set": changeMap}
	_, err := c.UpdateMany(context.TODO(), query, change)
	return err
}

// FindProductionServiceByApplicationLink finds production services linked to a specific application
func (c *ProductionServiceColl) FindProductionServiceByApplicationLink(applicationID primitive.ObjectID) ([]*models.Service, error) {
	query := bson.M{
		"application_id": applicationID,
		"status":         bson.M{"$ne": setting.ProductStatusDeleting},
	}

	var services []*models.Service
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.TODO(), &services)
	if err != nil {
		return nil, err
	}

	return services, nil
}

// CheckProductionServiceApplicationConflict checks if a production service is already linked to another application
func (c *ProductionServiceColl) CheckProductionServiceApplicationConflict(serviceName, productName string, applicationID primitive.ObjectID) (*models.Service, error) {
	query := bson.M{
		"service_name":   serviceName,
		"product_name":   productName,
		"application_id": bson.M{"$exists": true, "$nin": bson.A{nil, applicationID}},
		"status":         bson.M{"$ne": setting.ProductStatusDeleting},
	}

	var service models.Service
	err := c.FindOne(context.TODO(), query).Decode(&service)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // no conflict
		}
		return nil, err
	}

	return &service, nil
}
