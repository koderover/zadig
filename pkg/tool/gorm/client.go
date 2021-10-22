package gorm

import (
	"fmt"
	"sync"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/pkg/tool/log"
)

type Database int

const (
	Dex Database = iota + 1
)

var connections = make(map[Database]*gorm.DB, 1)
var once sync.Once

func SetExternalConnection(conn *gorm.DB) {
	once.Do(func() {
		connections[Dex] = conn
	})
}

func getConnection(db Database) *gorm.DB {
	once.Do(func() {
		connections[Dex] = initDex()
	})

	return connections[db]
}

func initDex() *gorm.DB {
	return openDB(
		config.DexMysqlUser(),
		config.DexMysqlPassword(),
		config.DexMysqlHost(),
		config.DexMysqlDB(),
	)
}

func DB(db ...Database) *gorm.DB {
	if len(db) == 0 {
		return getConnection(Dex)
	}
	return getConnection(db[0])
}

func Close() {
	//for _, conn := range connections {
	//	conn.Close()
	//}
}

func openDB(username, password, host, db string) *gorm.DB {

	// refer https://github.com/go-sql-driver/mysql#dsn-data-source-name for details
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		username, password, host, db,
	)
	conn, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Panicf("Can not open db %s, err: %s", db, err)
	}

	return conn
}
