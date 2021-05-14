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

package workflow

import (
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func CreateFavoritePipeline(args *commonmodels.Favorite, log *xlog.Logger) error {
	return commonrepo.NewFavoriteColl().Create(args)
}

func ListFavoritePipelines(args *commonrepo.FavoriteArgs) ([]*commonmodels.Favorite, error) {
	return commonrepo.NewFavoriteColl().List(args)
}

func DeleteFavoritePipeline(args *commonrepo.FavoriteArgs) error {
	return commonrepo.NewFavoriteColl().Delete(args)
}
