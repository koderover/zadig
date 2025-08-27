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

package models

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type NavigationItem struct {
	Name     string                    `bson:"name"       json:"name"`
	Key      config.NavigationItemKey  `bson:"key"        json:"key"`
	Type     config.NavigationItemType `bson:"type"       json:"type"`
	IconType string                    `bson:"icon_type"  json:"icon_type"`
	Icon     string                    `bson:"icon"       json:"icon"`
	PageType config.NavigationPageType `bson:"page_type"  json:"page_type"`
	URL      string                    `bson:"url"        json:"url"`
	Children []*NavigationItem         `bson:"children"   json:"children"`
}

type CustomNavigation struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"    json:"id,omitempty"`
	Items      []*NavigationItem  `bson:"items"            json:"items"`
	CreateTime int64              `bson:"create_time"       json:"create_time"`
	UpdateTime int64              `bson:"update_time"       json:"update_time"`
	UpdateBy   string             `bson:"update_by"         json:"update_by"`
}

func (CustomNavigation) TableName() string { return "custom_navigation" }

const (
	MaxNavigationDepth       = 10
	MaxNavigationTotalItems  = 1000
	MaxNavigationChildrenPer = 200
)

func (n *CustomNavigation) Validate() error {
	if n == nil {
		return fmt.Errorf("navigation is nil")
	}
	var total int
	var walk func(items []*NavigationItem, depth int) error
	walk = func(items []*NavigationItem, depth int) error {
		if depth > MaxNavigationDepth {
			return fmt.Errorf("navigation depth exceeds limit %d", MaxNavigationDepth)
		}
		if len(items) > MaxNavigationChildrenPer {
			return fmt.Errorf("children per node exceeds limit %d", MaxNavigationChildrenPer)
		}
		for _, it := range items {
			total++
			if total > MaxNavigationTotalItems {
				return fmt.Errorf("total items exceeds limit %d", MaxNavigationTotalItems)
			}
			if strings.TrimSpace(it.Name) == "" {
				return fmt.Errorf("navigation item name cannot be empty")
			}
			if strings.TrimSpace(string(it.Key)) == "" {
				return fmt.Errorf("navigation item key cannot be empty")
			}
			if it.Type != config.NavigationItemTypeFolder && it.Type != config.NavigationItemTypePage {
				return fmt.Errorf("invalid item type: %s", it.Type)
			}
			if it.Type == config.NavigationItemTypePage {
				if it.PageType != config.NavigationPageTypePlugin && it.PageType != config.NavigationPageTypeSystem {
					return fmt.Errorf("invalid pageType: %s", it.PageType)
				}
				if strings.TrimSpace(it.URL) == "" {
					return fmt.Errorf("url cannot be empty for page item")
				}
			} else {
				if strings.TrimSpace(string(it.PageType)) != "" || strings.TrimSpace(it.URL) != "" {
					return fmt.Errorf("folder item should not set pageType/url")
				}
			}
			if err := walk(it.Children, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(n.Items, 1)
}
