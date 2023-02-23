/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
)

const (
	FavoriteTypeEnv = "environment"
)

func CreateFavorite(favorite *models.Favorite) error {
	return mongodb.NewFavoriteColl().Create(favorite)
}

func DeleteFavorite(args *mongodb.FavoriteArgs) error {
	return mongodb.NewFavoriteColl().Delete(args)
}

func DeleteManyFavorites(args *mongodb.FavoriteArgs) error {
	return mongodb.NewFavoriteColl().DeleteManyByArgs(args)
}

func ListFavorites(args *mongodb.FavoriteArgs) ([]*models.Favorite, error) {
	return mongodb.NewFavoriteColl().List(args)
}
