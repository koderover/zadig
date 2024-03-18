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
