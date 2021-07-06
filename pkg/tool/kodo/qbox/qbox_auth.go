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

package qbox

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
)

// Mac 七牛AK/SK的对象，AK/SK可以从 https://portal.qiniu.com/user/key 获取。
type Mac struct {
	AccessKey string
	SecretKey []byte
}

// NewMac 构建一个新的拥有AK/SK的对象
func NewMac(accessKey, secretKey string) (mac *Mac) {
	return &Mac{accessKey, []byte(secretKey)}
}

// SignWithData 对数据进行签名，一般用于上传凭证的生成用途
func (mac *Mac) SignWithData(b []byte) (token string) {
	encodedData := base64.URLEncoding.EncodeToString(b)
	h := hmac.New(sha1.New, mac.SecretKey)
	h.Write([]byte(encodedData))
	digest := h.Sum(nil)
	sign := base64.URLEncoding.EncodeToString(digest)
	return fmt.Sprintf("%s:%s:%s", mac.AccessKey, sign, encodedData)
}
