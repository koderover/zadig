package orm

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"github.com/koderover/zadig/pkg/config"
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

func InitDBAndEditTables(action DbEditAction) error {
	var mysql []byte
	switch action {
	case DbEditActionAdd:
		mysql = addmysql
	case DbEditActionDrop:
		mysql = dropmysql
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
	_, err = db.Exec(string(mysql))
	if err != nil {
		return err
	}
	return nil
}
