//go:build darwin
// +build darwin

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

package gorm

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openDB(username, password, host, db string) (*gorm.DB, error) {
	// refer https://github.com/go-sql-driver/mysql#dsn-data-source-name for details
	if !config.MysqlUseDM() {
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			username, password, host, db,
		)
		return gorm.Open(mysql.Open(dsn), &gorm.Config{})
	} else {
		return nil, fmt.Errorf("dm database is not supported on darwin")
	}
}
