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

package middleware

import (
	"bytes"
	"encoding/json"
	"io/ioutil"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/x/log.v7"
)

type productName struct {
	ProductName string `json:"product_name"`
}

func StoreProductName(c *gin.Context) {
	pn := &productName{}
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("c.GetRawData() err : %v", err)
		return
	}
	if err = json.Unmarshal(data, pn); err != nil {
		log.Errorf("json.Unmarshal err : %v", err)
		return
	}
	c.Set("productName", pn.ProductName)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	c.Next()
}
