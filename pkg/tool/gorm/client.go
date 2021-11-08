package gorm

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var connections = make(map[string]*gorm.DB)

func getConnection(db string) *gorm.DB {
	return connections[db]
}

func Open(username, password, host, db string) error {
	conn, err := openDB(username, password, host, db)
	if err != nil {
		return err
	}

	connections[db] = conn

	return nil
}

func DB(db string) *gorm.DB {
	return getConnection(db)
}

func Close() {
	//for _, conn := range connections {
	//	conn.Close()
	//}
}

func openDB(username, password, host, db string) (*gorm.DB, error) {

	// refer https://github.com/go-sql-driver/mysql#dsn-data-source-name for details
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		username, password, host, db,
	)
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}
