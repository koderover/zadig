/*
Copyright 2022 The KodeRover Authors.

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

package orm

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"github.com/koderover/zadig/v2/pkg/config"
)

type DbEditAction string

const (
	DbEditActionAdd  DbEditAction = "add"
	DbEditActionDrop DbEditAction = "drop"
)

//go:embed addmysql.sql
var addmysql []byte

//go:embed dropmysql.sql
var dropmysql []byte

func UpdateUserDBTables(action DbEditAction) error {
	var mysql string
	switch action {
	case DbEditActionAdd:
		mysql = fmt.Sprintf(string(addmysql), config.MysqlUserDB())

	case DbEditActionDrop:
		mysql = fmt.Sprintf(string(dropmysql), config.MysqlUserDB())
	}
	if len(mysql) == 0 {
		return fmt.Errorf("%smysql.sql is empty", action)
	}
	db, err := sql.Open("mysql", fmt.Sprintf(
		"%s:%s@tcp(%s)/?charset=utf8&multiStatements=true",
		config.MysqlUser(), config.MysqlPassword(), config.MysqlHost(),
	))
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(mysql)
	return err
}

//go:embed alter_all_user.sql
var alterAllUserSQL []byte

func UpdateAllUserGroup() error {
	db, err := sql.Open("mysql", fmt.Sprintf(
		"%s:%s@tcp(%s)/?charset=utf8&multiStatements=true",
		config.MysqlUser(), config.MysqlPassword(), config.MysqlHost(),
	))
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(fmt.Sprintf(string(alterAllUserSQL), config.MysqlUserDB()))
	return err
}
