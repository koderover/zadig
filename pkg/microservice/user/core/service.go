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

package core

import (
	"context"
	_ "embed"

	_ "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	config2 "github.com/koderover/zadig/pkg/microservice/user/config"
	gormtool "github.com/koderover/zadig/pkg/tool/gorm"
)

var DB *gorm.DB

func Start(_ context.Context) {
	DB = gormtool.DB(config2.MysqlUserDB())
}
