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

package models

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Proxy struct {
	ID primitive.ObjectID `bson:"_id,omitempty"           json:"id,omitempty"`
	// http或socks5 暂时只支持http代理
	Type         string `bson:"type"                         json:"type"`
	Address      string `bson:"address"                      json:"address"`
	Port         int    `bson:"port"                         json:"port"`
	NeedPassword bool   `bson:"need_password"                json:"need_password"`
	Username     string `bson:"username"                     json:"username"`
	Password     string `bson:"password"                     json:"password"`
	// 代理用途，app表示应用代理，repo表示代码库代理。保留字段，暂时默认设置为default，以后可能用到。
	Usage                  string `bson:"usage"                        json:"usage"`
	EnableRepoProxy        bool   `bson:"enable_repo_proxy"            json:"enable_repo_proxy"`
	EnableApplicationProxy bool   `bson:"enable_application_proxy"     json:"enable_application_proxy"`
	CreateTime             int64  `bson:"create_time"                  json:"create_time"`
	UpdateTime             int64  `bson:"update_time"                  json:"update_time"`
	UpdateBy               string `bson:"update_by"                    json:"update_by"`
}

func (Proxy) TableName() string {
	return "proxy"
}

func (p *Proxy) GetProxyUrl() string {
	var uri string
	if p.NeedPassword {
		uri = fmt.Sprintf("%s://%s:%s@%s:%d",
			p.Type,
			p.Username,
			p.Password,
			p.Address,
			p.Port,
		)
	} else {
		uri = fmt.Sprintf("%s://%s:%d",
			p.Type,
			p.Address,
			p.Port,
		)
	}
	return uri
}
